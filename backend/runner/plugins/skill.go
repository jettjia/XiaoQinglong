package plugins

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/jettjia/XiaoQinglong/runner/pkg/logger"
	"github.com/jettjia/XiaoQinglong/runner/pkg/xqldir"
	"github.com/jettjia/XiaoQinglong/runner/tools"
	"github.com/jettjia/XiaoQinglong/runner/types"
)

// ========== Skill Runner ==========

// SkillRunner runs skills progressively with sliding-window approach
type SkillRunner struct {
	skills     map[string]types.Skill
	skillsDir  string // 存储所有 skill 脚本的目录
	sandboxCfg *types.SandboxConfig
	model      model.ToolCallingChatModel // 用于执行 skill 的模型
	configMgr  *SkillConfigManager        // skill 配置管理器

	// session 管理：多步骤 skill 执行时共享工作目录
	// key: sessionID, value: sessionWorkDir
	sessions map[string]string

	// CurrentSessionID 由 dispatcher 设置，标识当前请求的 session
	CurrentSessionID string

	// uploadsBaseDir 上传文件的基础目录，用于保存生成的报告
	uploadsBaseDir string

	// autoData 管理：存储脚本输出的 marker 数据（如 CHART_DATA_JSON）
	// key: sessionID, value: map[key]value (如 "CHART_DATA_JSON": "{...json...}")
	autoData map[string]map[string]string
}

// NewSkillRunner creates a new skill runner
func NewSkillRunner(skills []types.Skill, skillsDir string, sandboxCfg *types.SandboxConfig, model model.ToolCallingChatModel, configMgr *SkillConfigManager, uploadsBaseDir string) *SkillRunner {
	skillMap := make(map[string]types.Skill)
	for _, s := range skills {
		skillMap[s.ID] = s
	}
	return &SkillRunner{
		skills:         skillMap,
		skillsDir:      skillsDir,
		sandboxCfg:     sandboxCfg,
		model:          model,
		configMgr:      configMgr,
		uploadsBaseDir: uploadsBaseDir,
	}
}

// getReportsDir 获取报告保存目录：uploadsBaseDir/sessionID/reports
func (r *SkillRunner) getReportsDir() string {
	if r.uploadsBaseDir == "" || r.CurrentSessionID == "" {
		return "/tmp/reports"
	}
	dir := filepath.Join(r.uploadsBaseDir, r.CurrentSessionID, "reports")
	os.MkdirAll(dir, 0755)
	return dir
}

// getReportsURL 获取报告访问的 URL 基础路径
func (r *SkillRunner) getReportsURL() string {
	if r.CurrentSessionID == "" {
		return "/reports"
	}
	return fmt.Sprintf("/uploads/%s/reports", r.CurrentSessionID)
}

// SetAutoData 存储脚本输出的 marker 数据
func (r *SkillRunner) SetAutoData(sessionID string, key string, value string) {
	if r.autoData == nil {
		r.autoData = make(map[string]map[string]string)
	}
	if r.autoData[sessionID] == nil {
		r.autoData[sessionID] = make(map[string]string)
	}
	r.autoData[sessionID][key] = value
	logger.Infof("[SkillRunner] SetAutoData for session=%s, key=%s, valueLen=%d", sessionID, key, len(value))
}

// GetAutoData 获取存储的 marker 数据
func (r *SkillRunner) GetAutoData(sessionID string) map[string]string {
	if r.autoData == nil {
		return nil
	}
	return r.autoData[sessionID]
}

// RunSkill runs a skill with given input using sliding-window approach
// sessionID 用于标识同一个请求中的多次 skill 调用，实现工作目录共享
func (r *SkillRunner) RunSkill(ctx context.Context, name string, input map[string]any, sessionID string) (string, error) {
	skill, ok := r.skills[name]
	if !ok {
		return "", fmt.Errorf("skill not found: %s", name)
	}

	logger.Infof("[Skill] Running skill: %s, input: %v, sessionID: %s", name, input, sessionID)

	// 调试：检查 sandbox 配置
	if r.sandboxCfg != nil {
		logger.Infof("[Skill] sandbox enabled=%v, mode=%s, image=%s", r.sandboxCfg.Enabled, r.sandboxCfg.Mode, r.sandboxCfg.Image)
	} else {
		logger.Infof("[Skill] sandbox cfg is nil")
	}

	// 转换 input 为字符串
	var inputStr string
	if input != nil {
		if s, ok := input["query"].(string); ok && s != "" {
			inputStr = s
		} else if b, err := json.Marshal(input); err == nil {
			inputStr = string(b)
		}
	}
	if inputStr == "" {
		inputStr = name // 默认使用 skill name 作为 input
	}

	// 检查 skill 是否有脚本文件需要执行
	// 自动发现：如果 FilePath 或 EntryScript 未设置，尝试从 skill 目录自动发现
	skillDir := r.getSkillDir(skill.ID)
	discoveredFilePath, discoveredEntryScript := r.autoDiscoverScript(skillDir, skill.FilePath, skill.EntryScript)

	hasScript := discoveredFilePath != "" && discoveredEntryScript != ""

	// 如果自动发现了脚本，使用更新后的 skill 副本
	// 否则使用原始 skill（保持不变）
	skillForRun := skill
	if discoveredFilePath != "" {
		skillForRun.FilePath = discoveredFilePath
	}
	if discoveredEntryScript != "" {
		skillForRun.EntryScript = discoveredEntryScript
	}

	// 调试：检查 hasScript 状态
	logger.Infof("[Skill] hasScript=%v, sandboxCfg=%v, enabled=%v", hasScript, r.sandboxCfg != nil, r.sandboxCfg != nil && r.sandboxCfg.Enabled)

	// 如果沙箱启用且有配置，使用沙箱执行（即使没有脚本也可以使用沙箱中的工具）
	if r.sandboxCfg != nil && r.sandboxCfg.Enabled {
		// 使用沙箱执行 skill
		logger.Infof("[Skill] Using sandbox execution")
		return r.runSkillWithSandbox(ctx, skillForRun, inputStr, sessionID)
	}

	// 无沙箱时，使用简单的模型调用方式执行 skill
	logger.Infof("[Skill] Falling back to simple execution (sandbox disabled)")
	return r.runSkillSimple(ctx, skill, inputStr)
}

// runSkillWithSandbox 在沙箱中执行 skill（滑动窗口模式）
// sessionID 用于标识同一个请求中的多次 skill 调用，实现工作目录共享
func (r *SkillRunner) runSkillWithSandbox(ctx context.Context, skill types.Skill, input string, sessionID string) (string, error) {
	// 1. 准备 skill 文件目录
	skillDir := r.getSkillDir(skill.ID)
	if skillDir == "" {
		return "", fmt.Errorf("skill directory not found for: %s", skill.ID)
	}

	// 2. 初始化 sessions map
	if r.sessions == nil {
		r.sessions = make(map[string]string)
	}

	// 3. 创建或复用 session 工作目录
	var tmpDir string
	var err error

	if existingDir, ok := r.sessions[sessionID]; ok {
		// 复用已有 session 目录
		tmpDir = existingDir
		logger.Infof("[Skill] Reusing existing session workdir: %s for session: %s", tmpDir, sessionID)
	} else {
		// 首次创建 session 工作目录
		tmpDir, err = os.MkdirTemp("", "skill-session-*")
		if err != nil {
			return "", fmt.Errorf("create session dir failed: %w", err)
		}
		r.sessions[sessionID] = tmpDir
		logger.Infof("[Skill] Created new session workdir: %s for session: %s", tmpDir, sessionID)
	}

	// 4. 复制 skill 文件到工作目录
	if err := r.copySkillFiles(skillDir, tmpDir); err != nil {
		return "", fmt.Errorf("copy skill files failed: %w", err)
	}
	logger.Infof("[Skill] Copied skill files from %s to %s", skillDir, tmpDir)

	// 5. 创建沙箱工具：list_skill_files, read_skill_file, execute_skill_script_file
	tools := r.buildSkillSandboxTools(tmpDir, skill, input)
	logger.Infof("[Skill] Built %d sandbox tools", len(tools))

	// 5. 构建执行 instruction
	instruction := r.buildSkillInstruction(skill, input)
	logger.Infof("[Skill] Instruction length: %d", len(instruction))

	// 6. 使用 adk agent 渐进式执行
	logger.Infof("[Skill] Calling runSkillWithAgent...")
	return r.runSkillWithAgent(ctx, instruction, tools)
}

// runSkillWithAgent 使用 adk agent 渐进式执行 skill
func (r *SkillRunner) runSkillWithAgent(ctx context.Context, instruction string, tools []tool.BaseTool) (string, error) {
	if r.model == nil {
		return "", fmt.Errorf("model not configured for skill execution")
	}

	logger.Infof("[Skill] Creating skill executor agent with %d tools", len(tools))
	for i, t := range tools {
		info, _ := t.Info(ctx)
		if info != nil {
			logger.Infof("[Skill]   Tool[%d]: %s", i, info.Name)
		}
	}

	// 创建 skill 执行 agent
	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:          "skill_executor",
		Description:   "渐进式 Skill 执行器",
		Instruction:   instruction,
		Model:         r.model,
		MaxIterations: 30,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: tools,
			},
		},
	})
	if err != nil {
		logger.Errorf("[Skill] create skill agent failed: %v", err)
		return "", fmt.Errorf("create skill agent failed: %w", err)
	}
	logger.Infof("[Skill] Agent created successfully")

	// 设置超时（增加时间，因为需要调用多次工具）
	skillCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	// 运行 agent
	logger.Infof("[Skill] Starting agent execution...")
	runner := adk.NewRunner(skillCtx, adk.RunnerConfig{EnableStreaming: true, Agent: agent})
	logger.Infof("[Skill] Runner created, starting query...")
	iter := runner.Query(skillCtx, "执行任务")

	// 收集结果
	var lastContent string
	var eventCount int
	var hasToolCalls bool
	for {
		ev, ok := iter.Next()
		if !ok {
			break
		}
		eventCount++
		if ev == nil {
			logger.Infof("[Skill] Event[%d]: nil event", eventCount)
			continue
		}
		if ev.Output == nil {
			if ev.Err != nil {
				logger.Infof("[Skill] Event[%d]: ev.Output is nil, err=%v", eventCount, ev.Err)
			} else if ev.Action != nil {
				logger.Infof("[Skill] Event[%d]: ev.Output is nil, action=%+v", eventCount, ev.Action)
			} else {
				logger.Infof("[Skill] Event[%d]: ev.Output is nil, no err no action", eventCount)
			}
			continue
		}

		msgOut := ev.Output.MessageOutput
		if msgOut == nil {
			logger.Infof("[Skill] Event[%d]: ev.Output.MessageOutput is nil", eventCount)
			continue
		}

		// 检查 Message 中是否有 tool_calls
		if msgOut.Message != nil && len(msgOut.Message.ToolCalls) > 0 {
			hasToolCalls = true
			for _, tc := range msgOut.Message.ToolCalls {
				logger.Infof("[Skill] Event[%d]: tool_call - %s", eventCount, tc.Function.Name)
			}
		}

		// 处理 Message
		if msgOut.Message != nil && strings.TrimSpace(msgOut.Message.Content) != "" {
			lastContent = msgOut.Message.Content
			logger.Infof("[Skill] Event[%d]: got content, length=%d, hasToolCalls=%v", eventCount, len(lastContent), hasToolCalls)
		}

		// 处理 MessageStream (streaming)
		if msgOut.MessageStream != nil {
			for {
				chunk, err := msgOut.MessageStream.Recv()
				if err != nil {
					break
				}
				if chunk != nil && strings.TrimSpace(chunk.Content) != "" {
					lastContent += chunk.Content
					logger.Infof("[Skill] Event[%d]: stream chunk, length=%d", eventCount, len(chunk.Content))
				}
			}
		}
	}

	logger.Infof("[Skill] Agent execution completed, total events=%d, content length=%d, hasToolCalls=%v", eventCount, len(lastContent), hasToolCalls)

	// 后处理：检查内容是否包含未填充的 HTML 模板占位符
	finalContent := r.postProcessSkillOutput(lastContent, tools)
	if finalContent != lastContent {
		logger.Infof("[Skill] Post-processed HTML content, new length=%d", len(finalContent))
	}

	return finalContent, nil
}

// extractTemplateData 从 HTML 模板内容中提取模板路径和数据
func (r *SkillRunner) extractTemplateData(htmlContent string) (string, map[string]any) {
	// 从上次读取的文件路径记录中获取模板路径
	// 由于我们不知道确切路径，尝试常见的路径
	possiblePaths := []string{
		"csv-data-analysis/templates/report_template.html",
		"templates/report_template.html",
		"report_template.html",
	}

	var templatePath string
	for _, p := range possiblePaths {
		fullPath := filepath.Join(r.skillsDir, p)
		if _, err := os.Stat(fullPath); err == nil {
			templatePath = p
			break
		}
	}

	if templatePath == "" {
		// 尝试在工作目录中查找
		for _, p := range possiblePaths {
			if strings.Contains(htmlContent, p) || strings.Contains(htmlContent, filepath.Base(p)) {
				templatePath = p
				break
			}
		}
	}

	// 提取数据：从 HTML 内容中尝试提取可用的数据
	// 由于原始数据已丢失，只能返回空
	data := map[string]any{
		"LANG":                  "zh",
		"REPORT_TITLE":          "数据分析报告",
		"REPORT_SUBTITLE":       "自动生成",
		"EXEC_SUMMARY":          "数据概览完成",
		"DISTRIBUTION_INSIGHTS": "分布分析完成",
		"CORRELATION_INSIGHTS":  "相关性分析完成",
		"CATEGORICAL_INSIGHTS":  "分类分析完成",
		"TIME_SERIES_INSIGHTS":  "时序分析完成",
		"CONCLUSIONS":           "分析完成",
	}

	return templatePath, data
}

// readReportHTMLFile 根据报告 URL 读取实际的 HTML 文件内容
func (r *SkillRunner) readReportHTMLFile(reportURL string) string {
	// reportURL 格式: /uploads/{sessionID}/reports/report_xxx.html
	// 需要转换为实际的文件路径
	parts := strings.Split(reportURL, "/uploads/")
	if len(parts) < 2 {
		return ""
	}
	relPath := "uploads/" + parts[1]
	filePath := filepath.Join(r.uploadsBaseDir, relPath)
	data, err := os.ReadFile(filePath)
	if err != nil {
		logger.Infof("[Skill] readReportHTMLFile: failed to read %s: %v", filePath, err)
		return ""
	}
	return string(data)
}

// saveReportHTMLFile 保存 HTML 内容到报告文件
func (r *SkillRunner) saveReportHTMLFile(reportURL string, htmlContent string) {
	parts := strings.Split(reportURL, "/uploads/")
	if len(parts) < 2 {
		return
	}
	relPath := "uploads/" + parts[1]
	filePath := filepath.Join(r.uploadsBaseDir, relPath)
	if err := os.WriteFile(filePath, []byte(htmlContent), 0644); err != nil {
		logger.Infof("[Skill] saveReportHTMLFile: failed to write %s: %v", filePath, err)
		return
	}
	logger.Infof("[Skill] saveReportHTMLFile: saved report to %s", filePath)
}

// runSkillSimple 简单模式执行 skill（使用模型分析）
func (r *SkillRunner) runSkillSimple(ctx context.Context, skill types.Skill, input string) (string, error) {
	if r.model == nil {
		return "", fmt.Errorf("model not configured for skill execution")
	}

	// 构建分析 prompt：skill instruction + input (包含文件内容)
	prompt := fmt.Sprintf(`%s

请分析以下数据内容：

%s

请按照 skill instruction 中的要求执行分析任务。`, skill.Instruction, input)

	messages := []adk.Message{
		schema.SystemMessage("你是一个数据分析专家，擅长从数据中提取洞察并生成报告。"),
		schema.UserMessage(prompt),
	}

	resp, err := r.model.Generate(ctx, messages)
	if err != nil {
		return "", fmt.Errorf("model generate failed: %w", err)
	}

	return resp.Content, nil
}

// getSkillDir 获取 skill 目录
func (r *SkillRunner) getSkillDir(skillID string) string {
	skillsDir := r.skillsDir
	if skillsDir == "" {
		// 尝试使用统一的 skills 目录
		skillsDir = xqldir.GetSkillsDir()
	}
	// 尝试从 {skillsDir}/{skillID} 目录加载
	dir := filepath.Join(skillsDir, skillID)
	if info, err := os.Stat(dir); err == nil && info.IsDir() {
		return dir
	}
	// 返回空，调用方会处理错误
	return ""
}

// autoDiscoverScript 自动发现 skill 脚本
// 扫描 skill 目录下的 scripts/ 子目录，查找可执行脚本
func (r *SkillRunner) autoDiscoverScript(skillDir, existingFilePath, existingEntryScript string) (filePath, entryScript string) {
	filePath = existingFilePath
	entryScript = existingEntryScript

	// 如果已经有完整的配置，无需自动发现
	if filePath != "" && entryScript != "" {
		return
	}

	if skillDir == "" {
		return
	}

	// 扫描 scripts 目录
	scriptsDir := filepath.Join(skillDir, "scripts")
	if info, err := os.Stat(scriptsDir); err != nil || !info.IsDir() {
		return
	}

	// 查找 .py 和 .sh 脚本
	var pyFiles []string
	var shFiles []string

	filepath.Walk(scriptsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		ext := filepath.Ext(path)
		if ext == ".py" {
			pyFiles = append(pyFiles, path)
		} else if ext == ".sh" {
			shFiles = append(shFiles, path)
		}
		return nil
	})

	// 优先级：.py > .sh
	var foundScript string
	if len(pyFiles) > 0 {
		foundScript = pyFiles[0] // 取第一个 .py 文件
	} else if len(shFiles) > 0 {
		foundScript = shFiles[0] // 取第一个 .sh 文件
	}

	if foundScript == "" {
		return
	}

	// 计算相对于 skill 目录的路径
	relPath, err := filepath.Rel(skillDir, foundScript)
	if err != nil {
		return
	}

	// 设置 FilePath
	if filePath == "" {
		filePath = relPath
	}

	// 设置 EntryScript
	if entryScript == "" {
		ext := filepath.Ext(foundScript)
		if ext == ".py" {
			entryScript = "python3 " + filepath.Join("scripts", filepath.Base(foundScript))
		} else if ext == ".sh" {
			entryScript = "bash " + filepath.Join("scripts", filepath.Base(foundScript))
		}
	}

	return
}

// copySkillFiles 复制 skill 文件到目标目录
func (r *SkillRunner) copySkillFiles(srcDir, dstDir string) error {
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(dstDir, relPath)
		if info.IsDir() {
			return os.MkdirAll(targetPath, 0755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(targetPath, data, 0644)
	})
}

// buildSkillSandboxTools 构建沙箱工具
func (r *SkillRunner) buildSkillSandboxTools(workDir string, skill types.Skill, skillInput string) []tool.BaseTool {
	files := r.listSkillFiles(workDir)

	// list_skill_files 工具
	listTool := &skillListFilesTool{files: files}

	// read_skill_file 工具
	readTool := &skillReadFileTool{workDir: workDir}

	// exec_skill_command 工具
	var baseDir string
	if skill.FilePath != "" {
		baseDir = skill.FilePath
	}
	execTool := &skillExecCommandTool{
		workDir:    workDir,
		baseDir:    baseDir,
		sandboxCfg: r.sandboxCfg,
		dockerBin:  "docker",
		skillName:  skill.ID,
		configMgr:  r.configMgr,
		skillInput: skillInput,
	}

	// bash 工具：用于执行 CLI 命令（如 agent-browser）
	bashTool := &skillBashTool{
		workDir:    workDir,
		baseDir:    baseDir,
		sandboxCfg: r.sandboxCfg,
		dockerBin:  "docker",
		skillName:  skill.ID,
		configMgr:  r.configMgr,
		skillInput: skillInput,
	}

	tools := []tool.BaseTool{listTool, readTool, execTool, bashTool, NewHtmlInterpreterTool(workDir, r.getReportsDir(), r.getReportsURL())}

	// 对于 pptx skill，添加 pptxInterpreterTool
	if skill.ID == "pptx" {
		tools = append(tools, NewPptxInterpreterTool(workDir, r.getReportsDir(), r.getReportsURL()))
	}

	return tools
}

// buildSkillInstruction 构建 skill 执行 instruction
func (r *SkillRunner) buildSkillInstruction(skill types.Skill, input string) string {
	// 构建基础 instruction
	baseInstruction := fmt.Sprintf(`你是一个 Skill 执行助手。
当前 Skill 名称: %s
输入: %s
工作目录: %s

重要规则：
- 必须先读取 SKILL.md，理解 skill 的执行流程
- 使用 execute_skill_script_file 工具执行脚本，传入 skill_name、script_file_name 和 args 参数
- 使用 bash 工具执行 CLI 命令（如 agent-browser 等）
- 使用 html_interpreter 工具生成 HTML 报告（如果 SKILL.md 要求）
- html_interpreter 工具返回的 HTML 内容必须原样输出，不要总结或解释
- 最终输出只包含 HTML 内容，不要添加任何其他文字说明
- 不要先调用 list_skill_files`, skill.Name, input, r.skillsDir)

	// 针对 csv-data-analysis skill，提供具体的执行指引
	if skill.ID == "csv-data-analysis" {
		// 从 input 中提取文件路径
		filePath := extractFilePathFromInput(input)
		baseInstruction += fmt.Sprintf(`

【%s 的特殊执行指引】
脚本执行后会输出两部分内容：
1. [Statistical Summary] - 统计摘要，是纯文本
2. ###CHART_DATA_JSON_START###...###CHART_DATA_JSON_END### - 图表数据（JSON 格式）

执行步骤：
1. 调用 execute_skill_script_file 执行脚本：
   - skill_name=csv-data-analysis
   - script_file_name=csv_analyzer.py
   - args={"input_file": "%s"}
2. 从脚本输出中提取图表数据：
   - 找到 ###CHART_DATA_JSON_START### 和 ###CHART_DATA_JSON_END### 之间的内容
   - 这是一个 JSON 对象，包含 charts 数组等数据
3. 调用 html_interpreter 生成报告：
   - template_path="templates/report_template.html"
   - data 是 JSON 对象，包含以下10个字段：
     * LANG: "zh"
     * REPORT_TITLE: "数据分析报告"
     * REPORT_SUBTITLE: "基于CSV数据的自动分析"
     * EXEC_SUMMARY: 统计摘要的 HTML 格式
     * DISTRIBUTION_INSIGHTS: 分布分析洞察
     * CORRELATION_INSIGHTS: 相关性分析洞察
     * CATEGORICAL_INSIGHTS: 分类分析洞察
     * TIME_SERIES_INSIGHTS: 时序分析洞察
     * CONCLUSIONS: 结论与建议
     * CHART_DATA_JSON: 图表数据 JSON 字符串（从 ###CHART_DATA_JSON_START###...###CHART_DATA_JSON_END### 中提取的内容）
   - 从脚本输出的统计摘要中提取数据，填入上述字段

关键：html_interpreter 的 data 参数中：
- LANG 必须是字符串 "zh"
- 所有字段都必须是字符串（HTML 格式）
- CHART_DATA_JSON 必须是从脚本输出中提取的图表数据 JSON 字符串
- 如果某项无数据，填 "无数据"
- html_interpreter 返回 HTML 后直接输出，不要再调用任何工具`, skill.ID, filePath)
	}

	// 针对 pptx skill，提供具体的执行指引
	if skill.ID == "pptx" {
		baseInstruction += `

【pptx skill 的特殊执行指引】
使用 pptx_interpreter 工具直接生成 PPT 文件。

执行步骤：
1. 理解用户需求，规划 PPT 结构：
   - 封面页：标题、副标题、作者
   - 内容页：根据主题设计 3-10 页内容，每页包含标题和内容
   - 总结页：核心要点回顾
2. 调用 pptx_interpreter 工具生成 PPT：
   - title: PPT 标题
   - author: 作者（可选，默认 "AI Assistant"）
   - filename: 保存的文件名（可选，默认 "presentation_时间戳.pptx"）
   - slides: 幻灯片数组，每个 slide 是对象：
     * title: 幻灯片标题
     * content: 幻灯片内容（支持多行文本）
     * bg: 背景色（可选，如 "FFFFFF" 白色，"1E2761" 深蓝色）
     * title_color: 标题颜色（可选）
     * content_color: 内容颜色（可选）
     * title_size: 标题字号（可选，默认 36）
     * content_size: 内容字号（可选，默认 18）

3. pptx_interpreter 返回 PPT 文件的 URL 后直接输出该 URL，不要再调用任何工具

关键：
- slides 数组不能为空
- 每页内容要简洁，突出重点
- 建议使用深色背景（bg: "1E2761"）+ 白色文字做封面，内容页用浅色背景
- title 和 content 中的双引号需要转义
- 生成后直接输出 URL 即可，不需要额外解释`
	}

	// 针对 agent-browser skill，提供具体的执行指引
	if skill.ID == "agent-browser" {
		baseInstruction += `

【agent-browser 特殊执行指引】
执行步骤：
1. 使用 bash 工具调用 agent-browser open <url> 打开网页
2. 使用 bash 工具调用 agent-browser snapshot -i --json 获取页面元素快照
3. 根据页面元素，使用 bash 工具执行交互操作：
   - agent-browser click @eN     # 点击元素
   - agent-browser fill @eN "文本"  # 填写表单
   - agent-browser press Enter    # 按键
4. 页面变化后必须重新 snapshot 获取新元素
5. 使用 agent-browser get text @eN --json 提取文本数据
6. 使用 agent-browser get attr @eN "href" --json 提取链接

关键：
- 完成所有操作后，必须提取并返回用户需要的具体数据
- 返回内容应是自然语言描述的结果（如"北京今天气温25℃，多云"）
- 不要只返回工具调用的状态信息，要返回实际的查询结果
- 如果是搜索类任务，先 scroll 滚动页面找到数据，再用 get text 提取`
	}

	return baseInstruction
}

// extractFilePathFromInput 从 input JSON 中提取文件路径
func extractFilePathFromInput(input string) string {
	// 尝试解析 JSON 提取 input_file 或 file_path
	type inputStruct struct {
		InputFile string `json:"input_file"`
		FilePath  string `json:"file_path"`
		Task      string `json:"task"`
	}
	var data inputStruct
	if err := json.Unmarshal([]byte(input), &data); err == nil {
		if data.InputFile != "" {
			return data.InputFile
		}
		if data.FilePath != "" {
			return data.FilePath
		}
	}
	// 如果解析失败，返回原输入（可能包含路径信息）
	if len(input) > 200 {
		return input[:200] + "..."
	}
	return input
}

// ========== Skill Tools ==========

type skillListFilesTool struct {
	files []string
}

func (t *skillListFilesTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "list_skill_files",
		Desc: "列出 skill 包中的所有可用文件",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"skill_name": {Type: schema.String, Desc: "Skill 名称（可选）", Required: false},
		}),
	}, nil
}

func (t *skillListFilesTool) InvokableRun(ctx context.Context, argumentsInJSON string, opt ...tool.Option) (string, error) {
	logger.Infof("[skillListFilesTool] InvokableRun called with args: %s", argumentsInJSON)
	result := strings.Join(t.files, ", ")
	logger.Infof("[skillListFilesTool] Returning: %s", result)
	return result, nil
}

type skillReadFileTool struct {
	workDir string
}

func (t *skillReadFileTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "read_skill_file",
		Desc: "读取 skill 包中的文件内容",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"file_name": {Type: schema.String, Desc: "文件名", Required: true},
			"offset":    {Type: schema.String, Desc: "偏移量", Required: false},
			"limit":     {Type: schema.String, Desc: "限制字符数", Required: false},
		}),
	}, nil
}

func (t *skillReadFileTool) InvokableRun(ctx context.Context, argumentsInJSON string, opt ...tool.Option) (string, error) {
	logger.Infof("[skillReadFileTool] InvokableRun called with args: %s", argumentsInJSON)
	type req struct {
		FileName string `json:"file_name"`
		Offset   int    `json:"offset"`
		Limit    int    `json:"limit"`
	}
	var r req
	if err := json.Unmarshal([]byte(argumentsInJSON), &r); err != nil {
		return "", err
	}

	// 去掉 csv-data-analysis/ 前缀（如果存在），因为文件已被复制到 workDir 根目录
	fileName := strings.TrimPrefix(r.FileName, "csv-data-analysis/")
	fileName = strings.TrimPrefix(fileName, "csv-data-analysis\\")

	filePath := filepath.Join(t.workDir, fileName)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("file not found: %s", r.FileName)
	}

	content := string(data)
	offset := r.Offset
	limit := r.Limit

	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = 1500
	}

	runes := []rune(content)
	if offset >= len(runes) {
		return "", nil
	}

	end := offset + limit
	if end > len(runes) {
		end = len(runes)
	}

	result := string(runes[offset:end])
	logger.Infof("[skillReadFileTool] Returning content length: %d", len(result))
	return result, nil
}

type skillExecCommandTool struct {
	workDir    string
	baseDir    string
	sandboxCfg *types.SandboxConfig
	dockerBin  string
	skillName  string              // skill 名称，用于获取配置
	configMgr  *SkillConfigManager // 配置管理器
	skillInput string              // skill 输入参数，会传递给 SKILL_INPUT 环境变量
}

func (t *skillExecCommandTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "execute_skill_script_file",
		Desc: "执行 skill 目录下的脚本文件。输入脚本文件名和参数，脚本会在沙箱中执行并返回结果。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"skill_name":       {Type: schema.String, Desc: "Skill 名称", Required: true},
			"script_file_name": {Type: schema.String, Desc: "要执行的脚本文件名，如 csv_analyzer.py", Required: true},
			"args":             {Type: schema.Object, Desc: "传递给脚本的参数对象，如 {\"input_file\": \"/path/to/file.csv\"}", Required: false},
		}),
	}, nil
}

func (t *skillExecCommandTool) InvokableRun(ctx context.Context, argumentsInJSON string, opt ...tool.Option) (string, error) {
	logger.Infof("[execute_skill_script_file] Received arguments: %s", argumentsInJSON)

	type req struct {
		SkillName      string         `json:"skill_name"`
		ScriptFileName string         `json:"script_file_name"`
		Args           map[string]any `json:"args"`
	}
	var r req
	if err := json.Unmarshal([]byte(argumentsInJSON), &r); err != nil {
		return "", err
	}

	if r.SkillName == "" {
		return "", fmt.Errorf("skill_name is required")
	}
	if r.ScriptFileName == "" {
		return "", fmt.Errorf("script_file_name is required")
	}

	logger.Infof("[execute_skill_script_file] skill_name=%s, script_file_name=%s, args=%v", r.SkillName, r.ScriptFileName, r.Args)

	// 构建脚本路径：scripts/{script_file_name}（文件已被复制到 workDir 根目录）
	scriptRelPath := r.ScriptFileName
	scriptRelPath = strings.TrimPrefix(scriptRelPath, "scripts/")
	scriptRelPath = strings.TrimPrefix(scriptRelPath, "scripts\\")
	scriptRelPath = strings.TrimPrefix(scriptRelPath, "csv-data-analysis/scripts/")
	scriptRelPath = strings.TrimPrefix(scriptRelPath, "csv-data-analysis\\scripts\\")

	scriptPath := filepath.Join("scripts", scriptRelPath)
	fullScriptPath := filepath.Join(t.workDir, scriptPath)

	logger.Infof("[execute_skill_script_file] workDir=%s, fullScriptPath=%s", t.workDir, fullScriptPath)

	// 检查脚本是否存在
	if _, err := os.Stat(fullScriptPath); os.IsNotExist(err) {
		return "", fmt.Errorf("script file not found: %s", fullScriptPath)
	}

	// 构建 Python 命令
	var command string
	if r.Args != nil {
		argsJSON, _ := json.Marshal(r.Args)
		command = fmt.Sprintf("python3 %s '%s'", fullScriptPath, string(argsJSON))
	} else {
		command = fmt.Sprintf("python3 %s", fullScriptPath)
	}

	logger.Infof("[execute_skill_script_file] command: %s", command)

	// 使用沙箱执行命令
	if t.sandboxCfg != nil && t.sandboxCfg.Enabled {
		return t.execInSandbox(ctx, command)
	}

	// 本地执行
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = t.workDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("exec failed: %s", string(output))
	}

	outputText := strings.TrimSpace(string(output))
	// 提取 auto_data 并保存到文件
	t.extractAndSaveAutoData(outputText)
	return outputText, nil
}

// extractAndSaveAutoData 从脚本输出中提取 ###KEY_START###...###KEY_END### 标记数据并保存到文件
func (t *skillExecCommandTool) extractAndSaveAutoData(output string) {
	if output == "" || !strings.Contains(output, "###") {
		return
	}

	autoData := make(map[string]string)
	// 使用简单的字符串查找来提取 marker 数据
	// 查找所有 _START### 模式的位置
	startPattern := "_START###"
	for {
		idx := strings.Index(output, startPattern)
		if idx == -1 {
			break
		}
		// 向前找 ### 的位置，得到完整的 start marker
		startMarkerStart := strings.LastIndex(output[:idx], "###")
		if startMarkerStart == -1 {
			output = output[idx+len(startPattern):]
			continue
		}
		// 提取 key（在 ### 和 _START 之间的内容）
		keyStart := startMarkerStart + 3
		keyEnd := idx
		key := output[keyStart:keyEnd]
		if key == "" || strings.Contains(key, "###") {
			output = output[idx+len(startPattern):]
			continue
		}
		// 找对应的 end marker
		endMarker := "###" + key + "_END###"
		contentStart := idx + len(startPattern)
		endIdx := strings.Index(output[contentStart:], endMarker)
		if endIdx != -1 {
			value := strings.TrimSpace(output[contentStart : contentStart+endIdx])
			if value != "" {
				autoData[key] = value
			}
		}
		output = output[contentStart:]
	}

	if len(autoData) > 0 {
		// 保存到 workDir 下的 .auto_data.json 文件
		autoDataFile := filepath.Join(t.workDir, ".auto_data.json")
		dataJSON, err := json.Marshal(autoData)
		if err != nil {
			logger.Infof("[execute_skill_script_file] failed to marshal auto_data: %v", err)
			return
		}
		if err := os.WriteFile(autoDataFile, dataJSON, 0644); err != nil {
			logger.Infof("[execute_skill_script_file] failed to write auto_data file: %v", err)
			return
		}
		logger.Infof("[execute_skill_script_file] saved auto_data with keys: %v to %s", mapKeys(autoData), autoDataFile)
	}
}

func mapKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// ========== skillBashTool ==========
// skillBashTool 用于在沙箱中执行任意 bash 命令，支持 skills.sh 的 allowed-tools: Bash(...) 格式

type skillBashTool struct {
	workDir    string
	baseDir    string
	sandboxCfg *types.SandboxConfig
	dockerBin  string
	skillName  string
	configMgr  *SkillConfigManager
	skillInput string
}

func (t *skillBashTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "bash",
		Desc: "在沙箱中执行 bash 命令。用于执行 agent-browser 等 CLI 工具。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"command": {
				Type:        schema.String,
				Desc:        "要执行的 bash 命令",
				Required:    true,
			},
			"timeout": {
				Type:        schema.Integer,
				Desc:        "超时时间（秒），默认 30，最大 300",
				Required:    false,
			},
		}),
	}, nil
}

func (t *skillBashTool) ValidateInput(ctx context.Context, input string) *tools.ValidationResult {
	var bashInput struct {
		Command string `json:"command"`
		Timeout int    `json:"timeout,omitempty"`
	}
	if err := json.Unmarshal([]byte(input), &bashInput); err != nil {
		return &tools.ValidationResult{Valid: false, Message: fmt.Sprintf("invalid JSON: %v", err), ErrorCode: 1}
	}
	if bashInput.Command == "" {
		return &tools.ValidationResult{Valid: false, Message: "command is required", ErrorCode: 2}
	}
	return &tools.ValidationResult{Valid: true}
}

func (t *skillBashTool) InvokableRun(ctx context.Context, input string, opt ...tool.Option) (string, error) {
	var bashInput struct {
		Command string `json:"command"`
		Timeout int    `json:"timeout,omitempty"`
	}
	if err := json.Unmarshal([]byte(input), &bashInput); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	logger.Infof("[bash] command: %s", bashInput.Command)

	// 使用沙箱执行命令
	if t.sandboxCfg != nil && t.sandboxCfg.Enabled {
		return t.execInSandbox(ctx, bashInput.Command, bashInput.Timeout)
	}

	// 本地执行
	return t.execLocally(ctx, bashInput.Command, bashInput.Timeout)
}

// execInSandbox 在沙箱中执行命令
func (t *skillBashTool) execInSandbox(ctx context.Context, command string, timeoutSec int) (string, error) {
	if t.sandboxCfg == nil || !t.sandboxCfg.Enabled {
		return "", fmt.Errorf("sandbox is not enabled")
	}

	mode := t.sandboxCfg.Mode
	if mode == "" {
		mode = "docker"
	}

	if mode == "local" {
		return t.execLocally(ctx, command, timeoutSec)
	}

	// Docker 模式
	return t.execInDocker(ctx, command, timeoutSec)
}

// execLocally 本地执行命令
func (t *skillBashTool) execLocally(ctx context.Context, command string, timeoutSec int) (string, error) {
	workdir := t.sandboxCfg.Workdir
	if workdir == "" {
		workdir = t.workDir
	}

	baseDir := strings.TrimSpace(t.baseDir)
	baseForCommand := workdir
	if baseDir != "" {
		baseForCommand = filepath.Join(workdir, baseDir)
	}

	timeoutMs := t.sandboxCfg.TimeoutMs
	if timeoutMs <= 0 {
		timeoutMs = 60000
	}
	if timeoutSec > 0 && timeoutSec*1000 < timeoutMs {
		timeoutMs = timeoutSec * 1000
	}

	// 构建环境变量
	env := os.Environ()
	for k, v := range t.sandboxCfg.Env {
		kk := strings.TrimSpace(k)
		if kk == "" {
			continue
		}
		env = append(env, kk+"="+strings.TrimSpace(v))
	}

	if t.configMgr != nil && t.skillName != "" {
		skillEnvVars := t.configMgr.ToEnvVars(t.skillName)
		for k, v := range skillEnvVars {
			if k == "" {
				continue
			}
			env = append(env, k+"="+v)
		}
	}

	if t.skillInput != "" {
		env = append(env, "SKILL_INPUT="+t.skillInput)
	}

	callCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(callCtx, "sh", "-c", command)
	cmd.Dir = baseForCommand
	cmd.Env = env

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	stdoutText := strings.TrimSpace(stdout.String())
	stderrText := strings.TrimSpace(stderr.String())

	result := map[string]interface{}{
		"stdout":   stdoutText,
		"stderr":   stderrText,
		"exitCode": 0,
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result["exitCode"] = exitErr.ExitCode()
		} else {
			result["exitCode"] = -1
		}
		if stderrText == "" {
			result["stderr"] = err.Error()
		}
	}

	output, _ := json.Marshal(result)
	return string(output), nil
}

// execInDocker 在 Docker 容器中执行命令
func (t *skillBashTool) execInDocker(ctx context.Context, command string, timeoutSec int) (string, error) {
	dockerBin := t.dockerBin
	if dockerBin == "" {
		dockerBin = "docker"
	}

	if _, err := exec.LookPath(dockerBin); err != nil {
		return "", fmt.Errorf("docker command not found: %w", err)
	}

	image := t.sandboxCfg.Image
	if image == "" {
		image = "alpine:latest"
	}
	workdir := t.sandboxCfg.Workdir
	if workdir == "" {
		workdir = "/workspace"
	}
	if !strings.HasPrefix(workdir, "/") {
		workdir = "/" + workdir
	}

	timeoutMs := t.sandboxCfg.TimeoutMs
	if timeoutMs <= 0 {
		timeoutMs = 60000
	}
	if timeoutSec > 0 && timeoutSec*1000 < timeoutMs {
		timeoutMs = timeoutSec * 1000
	}
	network := t.sandboxCfg.Network

	baseDir := strings.TrimSpace(t.baseDir)
	baseForCommand := workdir
	if baseDir != "" {
		baseForCommand = strings.TrimRight(workdir, "/") + "/" + strings.TrimLeft(baseDir, "/")
	}

	command = strings.ReplaceAll(command, "{{.BaseDirectory}}", baseForCommand)

	// 构建 docker 命令
	args := []string{"run", "--rm", "--network", network}

	// 添加资源限制
	if t.sandboxCfg.Limits != nil {
		if t.sandboxCfg.Limits.CPU != "" {
			args = append(args, "--cpu", t.sandboxCfg.Limits.CPU)
		}
		if t.sandboxCfg.Limits.Memory != "" {
			args = append(args, "--memory", t.sandboxCfg.Limits.Memory)
		}
	}

	// 添加环境变量
	for k, v := range t.sandboxCfg.Env {
		if k != "" {
			args = append(args, "-e", k+"="+v)
		}
	}

	// 添加 skill 配置的环境变量
	if t.configMgr != nil && t.skillName != "" {
		skillEnvVars := t.configMgr.ToEnvVars(t.skillName)
		for k, v := range skillEnvVars {
			if k != "" {
				args = append(args, "-e", k+"="+v)
			}
		}
	}

	// 添加 SKILL_INPUT
	if t.skillInput != "" {
		args = append(args, "-e", "SKILL_INPUT="+t.skillInput)
	}

	// 设置工作目录
	args = append(args, "-w", baseForCommand)

	// 添加镜像和命令
	args = append(args, image, "sh", "-c", command)

	logger.Infof("[bash] docker command: docker %v", args)

	callCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(callCtx, dockerBin, args...)
	cmd.Env = os.Environ()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	stdoutText := strings.TrimSpace(stdout.String())
	stderrText := strings.TrimSpace(stderr.String())

	result := map[string]interface{}{
		"stdout":   stdoutText,
		"stderr":   stderrText,
		"exitCode": 0,
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result["exitCode"] = exitErr.ExitCode()
		} else {
			result["exitCode"] = -1
		}
		if stderrText == "" {
			result["stderr"] = err.Error()
		}
	}

	output, _ := json.Marshal(result)
	return string(output), nil
}

func (t *skillExecCommandTool) execInSandbox(ctx context.Context, command string) (string, error) {
	if t.sandboxCfg == nil || !t.sandboxCfg.Enabled {
		return "", fmt.Errorf("sandbox is not enabled")
	}

	// 根据 Mode 决定执行方式
	mode := t.sandboxCfg.Mode
	if mode == "" {
		mode = "docker" // 默认使用 docker
	}

	if mode == "local" {
		return t.execLocally(ctx, command)
	}

	// Docker 模式
	return t.execInDocker(ctx, command)
}

// execLocally 本地执行命令（不使用沙箱）
func (t *skillExecCommandTool) execLocally(ctx context.Context, command string) (string, error) {
	workdir := t.sandboxCfg.Workdir
	if workdir == "" {
		workdir = t.workDir
	}

	// 构建 baseDir 路径
	baseDir := strings.TrimSpace(t.baseDir)
	baseForCommand := workdir
	if baseDir != "" {
		baseForCommand = filepath.Join(workdir, baseDir)
	}

	// 替换命令中的占位符
	command = strings.ReplaceAll(command, "{{.BaseDirectory}}", baseForCommand)

	timeoutMs := t.sandboxCfg.TimeoutMs
	if timeoutMs <= 0 {
		timeoutMs = 60000
	}

	// 构建环境变量
	env := os.Environ()
	for k, v := range t.sandboxCfg.Env {
		kk := strings.TrimSpace(k)
		if kk == "" {
			continue
		}
		env = append(env, kk+"="+strings.TrimSpace(v))
	}

	// 添加 skill 专用配置的环境变量
	if t.configMgr != nil && t.skillName != "" {
		skillEnvVars := t.configMgr.ToEnvVars(t.skillName)
		for k, v := range skillEnvVars {
			if k == "" {
				continue
			}
			env = append(env, k+"="+v)
		}
	}

	// 添加 SKILL_INPUT 环境变量
	if t.skillInput != "" {
		env = append(env, "SKILL_INPUT="+t.skillInput)
	}

	// 执行命令
	callCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(callCtx, "sh", "-c", command)
	cmd.Dir = baseForCommand
	cmd.Env = env

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		stderrText := strings.TrimSpace(stderr.String())
		if stderrText == "" {
			stderrText = err.Error()
		}
		return "", fmt.Errorf("local exec failed: %s", stderrText)
	}

	stdoutText := strings.TrimSpace(stdout.String())
	stderrText := strings.TrimSpace(stderr.String())
	if stdoutText != "" {
		// 提取 auto_data 并保存到文件
		t.extractAndSaveAutoData(stdoutText)
		return stdoutText, nil
	}
	if stderrText != "" {
		return stderrText, nil
	}
	return "", nil
}

// execInDocker 在 Docker 容器中执行命令
func (t *skillExecCommandTool) execInDocker(ctx context.Context, command string) (string, error) {
	dockerBin := t.dockerBin
	if dockerBin == "" {
		dockerBin = "docker"
	}

	// 检查 docker 是否可用
	if _, err := exec.LookPath(dockerBin); err != nil {
		return "", fmt.Errorf("docker command not found: %w", err)
	}

	// 解析配置
	image := t.sandboxCfg.Image
	if image == "" {
		image = "alpine:latest"
	}
	workdir := t.sandboxCfg.Workdir
	if workdir == "" {
		workdir = "/workspace"
	}
	if !strings.HasPrefix(workdir, "/") {
		workdir = "/" + workdir
	}
	timeoutMs := t.sandboxCfg.TimeoutMs
	if timeoutMs <= 0 {
		timeoutMs = 60000
	}
	network := t.sandboxCfg.Network

	// 构建 baseDir 路径
	baseDir := strings.TrimSpace(t.baseDir)
	baseForCommand := workdir
	if baseDir != "" {
		baseForCommand = strings.TrimRight(workdir, "/") + "/" + strings.TrimLeft(baseDir, "/")
	}

	// 替换命令中的占位符
	command = strings.ReplaceAll(command, "{{.BaseDirectory}}", baseForCommand)

	// 替换宿主机的 workDir 路径为容器内的 workdir 路径
	// 因为宿主机路径在容器内不可见，只有挂载的 workdir 可用
	if t.workDir != workdir {
		command = strings.ReplaceAll(command, t.workDir, workdir)
	}

	// 生成容器名称
	containerName := fmt.Sprintf("skill-exec-%d", time.Now().UnixNano())

	logger.Infof("[execInDocker] image=%s, workdir=%s, command=%s", image, workdir, command)

	// 构建 docker run 参数
	args := []string{"run", "--rm", "--name", containerName, "-w", workdir}
	if network != "" {
		args = append(args, "--network", network)
	}

	// 添加资源限制
	if t.sandboxCfg.Limits != nil {
		if t.sandboxCfg.Limits.CPU != "" {
			args = append(args, "--cpus", t.sandboxCfg.Limits.CPU)
		}
		if t.sandboxCfg.Limits.Memory != "" {
			args = append(args, "--memory", t.sandboxCfg.Limits.Memory)
		}
	}

	// 添加环境变量
	for k, v := range t.sandboxCfg.Env {
		kk := strings.TrimSpace(k)
		if kk == "" {
			continue
		}
		args = append(args, "-e", kk+"="+strings.TrimSpace(v))
	}

	// 添加 skill 专用配置的环境变量
	if t.configMgr != nil && t.skillName != "" {
		skillEnvVars := t.configMgr.ToEnvVars(t.skillName)
		for k, v := range skillEnvVars {
			if k == "" {
				continue
			}
			// 直接传递环境变量，不加前缀
			args = append(args, "-e", k+"="+v)
		}
	}

	// 添加 SKILL_INPUT 环境变量
	if t.skillInput != "" {
		args = append(args, "-e", "SKILL_INPUT="+t.skillInput)
	}

	// 添加工作目录挂载
	args = append(args, "-v", fmt.Sprintf("%s:%s", t.workDir, workdir))

	// 添加额外的卷挂载（如 uploads 目录）
	if t.sandboxCfg != nil && len(t.sandboxCfg.Volumes) > 0 {
		for _, vol := range t.sandboxCfg.Volumes {
			mode := "rw"
			if vol.ReadOnly {
				mode = "ro"
			}
			args = append(args, "-v", fmt.Sprintf("%s:%s:%s", vol.HostPath, vol.ContainerPath, mode))
		}
	}

	args = append(args, "--entrypoint", "", image, "sh", "-c", command)

	logger.Infof("[execInDocker] docker args: %v", args)

	// 执行命令
	callCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(callCtx, dockerBin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		stderrText := strings.TrimSpace(stderr.String())
		if stderrText == "" {
			stderrText = err.Error()
		}
		logger.Errorf("[execInDocker] failed: %s, stderr: %s", err, stderrText)
		// 清理容器
		exec.Command(dockerBin, "rm", "-f", containerName).Run()
		return "", fmt.Errorf("sandbox exec failed: %s", stderrText)
	}

	// 清理容器
	exec.Command(dockerBin, "rm", "-f", containerName).Run()

	stdoutText := strings.TrimSpace(stdout.String())
	stderrText := strings.TrimSpace(stderr.String())
	if stdoutText != "" {
		// 提取 auto_data 并保存到文件
		t.extractAndSaveAutoData(stdoutText)
		return stdoutText, nil
	}
	if stderrText != "" {
		return stderrText, nil
	}
	return "", nil
}

// ========== htmlInterpreterTool ==========

// htmlInterpreterTool HTML 模板解释器工具
type htmlInterpreterTool struct {
	workDir    string // 工作目录，用于解析模板路径
	reportsDir string // HTML 报告输出目录
	baseURL    string // 报告访问的基础 URL
}

// NewHtmlInterpreterTool 创建 HTML 解释器工具
// reportsDir: 报告保存的目录（通常是 uploads/sessionID）
// reportsURL: 报告访问的 URL 基础路径
func NewHtmlInterpreterTool(workDir, reportsDir, reportsURL string) *htmlInterpreterTool {
	os.MkdirAll(reportsDir, 0755)
	return &htmlInterpreterTool{
		workDir:    workDir,
		reportsDir: reportsDir,
		baseURL:    reportsURL,
	}
}

func (t *htmlInterpreterTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "html_interpreter",
		Desc: "将数据注入 HTML 模板，生成完整的 HTML 报告。使用方法：1) template_path 传入模板路径如 csv-data-analysis/templates/report_template.html；2) data 传入 JSON 对象，必须包含 LANG(如zh/en)、REPORT_TITLE、REPORT_SUBTITLE、EXEC_SUMMARY、DISTRIBUTION_INSIGHTS、CORRELATION_INSIGHTS、CATEGORICAL_INSIGHTS、TIME_SERIES_INSIGHTS、CONCLUSIONS 等字段。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"template_path": {Type: schema.String, Desc: "模板文件路径，如 csv-data-analysis/templates/report_template.html", Required: true},
			"data":          {Type: schema.Object, Desc: "JSON对象，必须包含: LANG(语言zh/en)、REPORT_TITLE(报告标题)、REPORT_SUBTITLE(副标题)、EXEC_SUMMARY(执行摘要)、DISTRIBUTION_INSIGHTS、CORRELATION_INSIGHTS、CATEGORICAL_INSIGHTS、TIME_SERIES_INSIGHTS、CONCLUSIONS", Required: true},
		}),
	}, nil
}

func (t *htmlInterpreterTool) InvokableRun(ctx context.Context, argumentsInJSON string, opt ...tool.Option) (string, error) {
	type req struct {
		TemplatePath string `json:"template_path"`
		Data         any    `json:"data"` // 使用 any 以接收字符串或对象
	}
	var r req
	if err := json.Unmarshal([]byte(argumentsInJSON), &r); err != nil {
		return "", fmt.Errorf("parse arguments failed: %w", err)
	}

	if r.TemplatePath == "" {
		return "", fmt.Errorf("template_path is required")
	}

	// 处理 data：可能是对象或 JSON 字符串
	var data map[string]any
	switch v := r.Data.(type) {
	case map[string]any:
		data = v
		// 检查是否缺少 CHART_DATA_JSON，如果是，尝试从其他字段提取
		if _, hasChartData := data["CHART_DATA_JSON"]; !hasChartData {
			logger.Infof("[htmlInterpreterTool] CHART_DATA_JSON not found in data, attempting to extract from string fields")
			if chartData := extractChartDataFromMapFields(data); chartData != "" {
				data["CHART_DATA_JSON"] = chartData
				logger.Infof("[htmlInterpreterTool] Extracted CHART_DATA_JSON from map fields, length=%d", len(chartData))
			}
		}
	case string:
		// 如果是字符串，尝试解析为 JSON 对象
		if err := json.Unmarshal([]byte(v), &data); err != nil {
			// 如果解析失败，说明传入的是原始文本内容
			// 尝试从 csv-data-analysis 原始输出中提取数据和洞察
			logger.Warnf("[htmlInterpreterTool] data string parse failed: %v, trying csv-data-analysis extraction", err)
			insights := ExtractCsvInsightsFromRawText(v)
			data = insights.ToMap()
			logger.Infof("[htmlInterpreterTool] Extracted csv-data-analysis insights, chart data length=%d", len(insights.ChartDataJSON))
		}
	default:
		return "", fmt.Errorf("data must be object or JSON string, got %T", r.Data)
	}

	// 尝试从 .auto_data.json 文件读取 auto_data（如 CHART_DATA_JSON）
	// 这是 skillExecCommandTool 在执行脚本后提取并保存的 marker 数据
	autoDataFile := filepath.Join(t.workDir, ".auto_data.json")
	if autoDataBytes, err := os.ReadFile(autoDataFile); err == nil {
		var autoData map[string]string
		if err := json.Unmarshal(autoDataBytes, &autoData); err == nil {
			for k, v := range autoData {
				keyUpper := strings.ToUpper(k)
				placeholderName := keyUpper
				// 对于 CHART_DATA_JSON 等特殊 key，确保名称匹配
				if _, exists := data[placeholderName]; !exists {
					// 将 auto_data 的 key 转换为大写作为占位符名
					data[placeholderName] = v
					logger.Infof("[htmlInterpreterTool] Loaded auto_data: %s, len=%d", placeholderName, len(v))
				}
			}
		}
	}

	// 读取模板文件
	templatePath := r.TemplatePath
	// 去掉 csv-data-analysis/ 前缀（如果存在），因为文件已被复制到 workDir 根目录
	templatePath = strings.TrimPrefix(templatePath, "csv-data-analysis/")
	templatePath = strings.TrimPrefix(templatePath, "csv-data-analysis\\")
	if !filepath.IsAbs(templatePath) {
		templatePath = filepath.Join(t.workDir, templatePath)
	}
	templateContent, err := os.ReadFile(templatePath)
	if err != nil {
		return "", fmt.Errorf("read template failed: %w", err)
	}

	// 注入数据到模板
	html := string(templateContent)

	// 保存原始模板的引用，用于后续检查 {{CHART_DATA_JSON}} 是否仍存在
	hasChartDataPlaceholder := strings.Contains(html, "{{CHART_DATA_JSON}}")

	// 替换占位符
	for key, value := range data {
		placeholder := "{{" + key + "}}"
		var replacement string
		switch v := value.(type) {
		case string:
			replacement = v
		case float64:
			replacement = fmt.Sprintf("%v", v)
		case bool:
			replacement = fmt.Sprintf("%v", v)
		default:
			// 对于复杂对象，序列化为 JSON
			if jsonBytes, err := json.Marshal(v); err == nil {
				replacement = string(jsonBytes)
			} else {
				replacement = fmt.Sprintf("%v", v)
			}
		}
		html = strings.ReplaceAll(html, placeholder, replacement)
	}

	// 如果 {{CHART_DATA_JSON}} 仍然存在，尝试从 data 中再次提取
	if hasChartDataPlaceholder && strings.Contains(html, "{{CHART_DATA_JSON}}") {
		logger.Infof("[htmlInterpreterTool] CHART_DATA_JSON placeholder still unfilled, attempting recovery")
		// 尝试从 data 的字符串字段中提取
		if chartData := extractChartDataFromMapFields(data); chartData != "" {
			html = strings.ReplaceAll(html, "{{CHART_DATA_JSON}}", chartData)
			logger.Infof("[htmlInterpreterTool] Recovered chart data and injected, length=%d", len(chartData))
		} else {
			// 最后尝试：从 data 中找到任何包含 JSON 数组的字符串（charts 通常是数组）
			for _, v := range data {
				if str, ok := v.(string); ok {
					if strings.Contains(str, "[") && strings.Contains(str, "charts") {
						// 尝试提取 charts 数组
						chartData := extractChartDataFromMarkers(str)
						if chartData != "" {
							html = strings.ReplaceAll(html, "{{CHART_DATA_JSON}}", chartData)
							logger.Infof("[htmlInterpreterTool] Extracted charts from field content, length=%d", len(chartData))
							break
						}
					}
				}
			}
		}
	}

	logger.Infof("[htmlInterpreterTool] Generated HTML report, length=%d", len(html))

	// 保存 HTML 到文件
	filename := fmt.Sprintf("report_%d.html", time.Now().UnixNano())
	filePath := filepath.Join(t.reportsDir, filename)
	os.WriteFile(filePath, []byte(html), 0644)
	logger.Infof("[htmlInterpreterTool] HTML report saved to: %s", filePath)

	// 返回报告 URL（直接返回路径，前端会识别 /uploads/.../reports/*.html 格式）
	reportURL := t.baseURL + "/" + filename
	return reportURL, nil
}

// listSkillFiles 列出 skill 目录下的所有文件
func (r *SkillRunner) listSkillFiles(dir string) []string {
	var files []string
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			return nil
		}
		files = append(files, relPath)
		return nil
	})
	return files
}

// ========== External Skill Tool (for agent) ==========

// BuildSkillTool creates a tool for running skills
func (r *SkillRunner) BuildSkillTool() tool.BaseTool {
	return &skillTool{
		runner: r,
	}
}

type skillTool struct {
	runner *SkillRunner
}

func (t *skillTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	var skillNames []string
	for name := range t.runner.skills {
		skillNames = append(skillNames, name)
	}

	// 如果没有注册任何 skill，不注册 run_skill 工具
	if len(skillNames) == 0 {
		return nil, nil
	}

	params := schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
		"name": {
			Type:     schema.String,
			Desc:     "The skill name to run",
			Enum:     skillNames,
			Required: true,
		},
		"input": {
			Type:     schema.Object,
			Desc:     "Input parameters for the skill (e.g., {query: \"...\"})",
			Required: false,
		},
	})

	desc := "Run a skill. Available skills: " + joinStrings(skillNames, ", ")

	return &schema.ToolInfo{
		Name:        "run_skill",
		Desc:        desc,
		ParamsOneOf: params,
	}, nil
}

func (t *skillTool) InvokableRun(ctx context.Context, argumentsInJSON string, opt ...tool.Option) (string, error) {
	type skillInput struct {
		Name  string `json:"name"`
		Input any    `json:"input"`
	}

	var input skillInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", err
	}

	// 转换 input 为 map[string]any
	var inputMap map[string]any
	if input.Input != nil {
		if m, ok := input.Input.(map[string]any); ok {
			inputMap = m
		} else if s, ok := input.Input.(string); ok && s != "" {
			// 如果是字符串，尝试解析为 JSON
			if err := json.Unmarshal([]byte(s), &inputMap); err != nil {
				inputMap = map[string]any{"query": s}
			}
		}
	}

	return t.runner.RunSkill(ctx, input.Name, inputMap, t.runner.CurrentSessionID)
}

// BuildLoadSkillTool creates a tool for loading skill details on-demand
func (r *SkillRunner) BuildLoadSkillTool() tool.BaseTool {
	return &loadSkillTool{
		runner: r,
	}
}

type loadSkillTool struct {
	runner *SkillRunner
}

func (t *loadSkillTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	var skillNames []string
	for name := range t.runner.skills {
		skillNames = append(skillNames, name)
	}

	// 如果没有注册任何 skill，不注册 load_skill 工具
	if len(skillNames) == 0 {
		return nil, nil
	}

	return &schema.ToolInfo{
		Name: "load_skill",
		Desc: "按需加载 skill 的完整内容，用于查看 skill 详细说明和用法",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"name": {
				Type:     schema.String,
				Desc:     "Skill 名称",
				Enum:     skillNames,
				Required: true,
			},
		}),
	}, nil
}

func (t *loadSkillTool) InvokableRun(ctx context.Context, argumentsInJSON string, opt ...tool.Option) (string, error) {
	type req struct {
		Name string `json:"name"`
	}
	var r req
	if err := json.Unmarshal([]byte(argumentsInJSON), &r); err != nil {
		return "", err
	}

	skill, ok := t.runner.skills[r.Name]
	if !ok {
		// 列出可用 skills
		var names []string
		for k := range t.runner.skills {
			names = append(names, k)
		}
		return fmt.Sprintf("未知 skill: %s\n可用 skills: %s", r.Name, joinStrings(names, ", ")), nil
	}

	// 返回完整 instruction
	return fmt.Sprintf("<skill name=\"%s\">\n%s\n</skill>", skill.Name, skill.Instruction), nil
}

// BuildSkillOrchestratorTool creates a tool for orchestrating multiple skills with LLM planning
func (r *SkillRunner) BuildSkillOrchestratorTool(planner *SkillPlanner) tool.BaseTool {
	return &skillOrchestratorTool{
		runner:  r,
		planner: planner,
	}
}

type skillOrchestratorTool struct {
	runner  *SkillRunner
	planner *SkillPlanner
}

func (t *skillOrchestratorTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	var skillNames []string
	for name := range t.runner.skills {
		skillNames = append(skillNames, name)
	}

	// 如果没有注册任何 skill，不注册 orchestrate_skills 工具
	if len(skillNames) == 0 {
		return nil, nil
	}

	params := schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
		"task": {
			Type:     schema.String,
			Desc:     "用户需求任务描述，系统会自动规划需要执行哪些 skill 以及顺序",
			Required: true,
		},
		"context": {
			Type:     schema.Object,
			Desc:     "执行任务时需要的上下文信息 (可选)",
			Required: false,
		},
	})

	desc := "智能编排多个 skill：系统会根据用户需求自动分析需要哪些 skill，以最优顺序执行。支持并行执行无依赖的 skill。"

	return &schema.ToolInfo{
		Name:        "orchestrate_skills",
		Desc:        desc,
		ParamsOneOf: params,
	}, nil
}

func (t *skillOrchestratorTool) InvokableRun(ctx context.Context, argumentsInJSON string, opt ...tool.Option) (string, error) {
	type req struct {
		Task    string         `json:"task"`
		Context map[string]any `json:"context"`
	}

	var r req
	if err := json.Unmarshal([]byte(argumentsInJSON), &r); err != nil {
		return "", err
	}

	if r.Context == nil {
		r.Context = make(map[string]any)
	}

	// 1. LLM 规划执行计划
	plan, err := t.planner.PlanWithLLM(ctx, r.Task, r.Context)
	if err != nil {
		return "", fmt.Errorf("skill planning failed: %w", err)
	}

	if len(plan.Stages) == 0 {
		return "未找到需要执行的 skills", nil
	}

	// 2. 执行计划
	results, err := t.planner.Execute(ctx, plan)
	if err != nil {
		return "", fmt.Errorf("skill execution failed: %w", err)
	}

	// 3. 汇总结果
	return formatSkillResults(results), nil
}

func formatSkillResults(results map[string]any) string {
	var lines []string
	for k, v := range results {
		lines = append(lines, fmt.Sprintf("%s: %v", k, v))
	}
	if len(lines) == 0 {
		return "执行完成，无结果"
	}
	return joinStrings(lines, "\n")
}

// SkillListText 生成 Layer 1 的简短 skill 列表
// 每个 skill 的描述限制在 MaxSkillDescChars 字符内（参考 Claude Code 的 MAX_LISTING_DESC_CHARS = 250）
const MaxSkillDescChars = 250

func (r *SkillRunner) SkillListText() string {
	if len(r.skills) == 0 {
		return "(无可用skills)"
	}

	var items []string
	var total int
	maxRunes := 1000

	for name, skill := range r.skills {
		desc := skill.Description
		if desc == "" {
			desc = "无描述"
		}
		// 截断过长的描述，限制在 MaxSkillDescChars 字符内
		if len([]rune(desc)) > MaxSkillDescChars {
			desc = string([]rune(desc)[:MaxSkillDescChars-1]) + "…"
		}

		item := fmt.Sprintf("%s: %s", name, desc)
		if total+len([]rune(item)) > maxRunes {
			break
		}
		items = append(items, item)
		total += len([]rune(item))
	}

	if len(items) == 0 {
		return "(无可用skills)"
	}
	return joinStrings(items, "; ")
}

func joinStrings(s []string, sep string) string {
	if len(s) == 0 {
		return ""
	}
	result := s[0]
	for i := 1; i < len(s); i++ {
		result += sep + s[i]
	}
	return result
}
