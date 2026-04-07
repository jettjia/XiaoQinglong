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
}

// NewSkillRunner creates a new skill runner
func NewSkillRunner(skills []types.Skill, skillsDir string, sandboxCfg *types.SandboxConfig, model model.ToolCallingChatModel, configMgr *SkillConfigManager) *SkillRunner {
	skillMap := make(map[string]types.Skill)
	for _, s := range skills {
		skillMap[s.ID] = s
	}
	return &SkillRunner{
		skills:     skillMap,
		skillsDir:  skillsDir,
		sandboxCfg: sandboxCfg,
		model:      model,
		configMgr:  configMgr,
	}
}

// RunSkill runs a skill with given input using sliding-window approach
// sessionID 用于标识同一个请求中的多次 skill 调用，实现工作目录共享
func (r *SkillRunner) RunSkill(ctx context.Context, name string, input map[string]any, sessionID string) (string, error) {
	skill, ok := r.skills[name]
	if !ok {
		return "", fmt.Errorf("skill not found: %s", name)
	}

	logger.Infof((fmt.Sprintf("[Skill] Running skill: %s, input: %v, sessionID: %s", name, input, sessionID)))

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

	if hasScript && r.sandboxCfg != nil && r.sandboxCfg.Enabled {
		// 使用沙箱执行 skill
		logger.Infof("[Skill] Using sandbox execution")
		return r.runSkillWithSandbox(ctx, skillForRun, inputStr, sessionID)
	}

	// 无沙箱时，使用简单的模型调用方式执行 skill
	logger.Infof("[Skill] Falling back to simple execution (sandbox disabled or no script)")
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
		return "", fmt.Errorf("create skill agent failed: %w", err)
	}

	// 设置超时
	skillCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	// 运行 agent
	logger.Infof("[Skill] Starting agent execution...")
	runner := adk.NewRunner(skillCtx, adk.RunnerConfig{EnableStreaming: false, Agent: agent})
	iter := runner.Query(skillCtx, "请执行任务")

	// 收集结果
	var lastContent string
	for {
		ev, ok := iter.Next()
		if !ok {
			break
		}
		if ev == nil || ev.Output == nil || ev.Output.MessageOutput == nil {
			continue
		}
		mo := ev.Output.MessageOutput
		if mo.Message != nil && strings.TrimSpace(mo.Message.Content) != "" {
			lastContent = mo.Message.Content
		}
	}

	logger.Infof("[Skill] Agent execution completed, content length: %d", len(lastContent))
	if lastContent == "" {
		return "", fmt.Errorf("skill execution returned no content")
	}

	return lastContent, nil
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
	if r.skillsDir == "" {
		return ""
	}
	// 尝试从 ./skills/{skillID} 目录加载
	dir := filepath.Join(r.skillsDir, skillID)
	if info, err := os.Stat(dir); err == nil && info.IsDir() {
		return dir
	}
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

	return []tool.BaseTool{listTool, readTool, execTool, NewHtmlInterpreterTool(workDir)}
}

// buildSkillInstruction 构建 skill 执行 instruction
func (r *SkillRunner) buildSkillInstruction(skill types.Skill, input string) string {
	// 构建基础 instruction
	baseInstruction := fmt.Sprintf(`你是一个 Skill 执行助手。
当前 Skill 名称: %s
输入: %s
工作目录: %s

执行步骤：
1. 首先调用 read_skill_file 读取 SKILL.md，这是最重要的入口文件
2. 仔细阅读 SKILL.md 中的指引，了解脚本结构和执行方式
3. 读取 scripts 目录下的脚本文件，了解其用法
4. 根据 SKILL.md 的指引，使用 execute_skill_script_file 工具执行脚本
5. 使用 html_interpreter 工具（如适用）生成 HTML 报告
6. 返回执行结果

重要规则：
- 必须先读取 SKILL.md，理解 skill 的执行流程
- 使用 execute_skill_script_file 工具执行脚本，传入 skill_name、script_file_name 和 args 参数
- 使用 html_interpreter 工具生成 HTML 报告（如果 SKILL.md 要求）
- 不要先调用 list_skill_files`, skill.Name, input, r.skillsDir)

	// 针对 csv-data-analysis skill，提供具体的执行指引
	if skill.ID == "csv-data-analysis" {
		// 从 input 中提取文件路径
		filePath := extractFilePathFromInput(input)
		baseInstruction += fmt.Sprintf(`

【%s 的特殊执行指引】
1. 脚本位置: scripts/csv_analyzer.py
2. 执行命令格式: python3 scripts/csv_analyzer.py '{"input_file": "<文件路径>"}'
3. 如果输入中包含 file_path，直接使用该路径
4. 示例: python3 scripts/csv_analyzer.py '{"input_file": "%s"}'
5. 执行完脚本后，使用 html_interpreter 工具生成报告
6. html_interpreter 的 template_path 参数: csv-data-analysis/templates/report_template.html
7. 注意：html_interpreter 工具已经可用，无需注册`, skill.ID, filePath)
	}

	return baseInstruction
}

// extractFilePathFromInput 从 input JSON 中提取文件路径
func extractFilePathFromInput(input string) string {
	// 尝试解析 JSON 提取 file_path
	type inputStruct struct {
		FilePath string `json:"file_path"`
		Task     string `json:"task"`
	}
	var data inputStruct
	if err := json.Unmarshal([]byte(input), &data); err == nil && data.FilePath != "" {
		return data.FilePath
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
	return strings.Join(t.files, ", "), nil
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
	type req struct {
		FileName string `json:"file_name"`
		Offset   int    `json:"offset"`
		Limit    int    `json:"limit"`
	}
	var r req
	if err := json.Unmarshal([]byte(argumentsInJSON), &r); err != nil {
		return "", err
	}

	filePath := filepath.Join(t.workDir, r.FileName)
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

	return string(runes[offset:end]), nil
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

	// 构建脚本路径：skills/{skill_name}/scripts/{script_file_name}
	scriptRelPath := r.ScriptFileName
	scriptRelPath = strings.TrimPrefix(scriptRelPath, "scripts/")
	scriptRelPath = strings.TrimPrefix(scriptRelPath, "scripts\\")

	scriptPath := filepath.Join("skills", r.SkillName, "scripts", scriptRelPath)
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
	workDir string // 工作目录，用于解析模板路径
}

// NewHtmlInterpreterTool 创建 HTML 解释器工具
func NewHtmlInterpreterTool(workDir string) *htmlInterpreterTool {
	return &htmlInterpreterTool{workDir: workDir}
}

func (t *htmlInterpreterTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "html_interpreter",
		Desc: "将数据注入 HTML 模板，生成完整的 HTML 报告。输入模板路径和数据 JSON，输出渲染后的 HTML 字符串。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"template_path": {Type: schema.String, Desc: "模板文件路径，如 csv-data-analysis/templates/report_template.html", Required: true},
			"data":          {Type: schema.String, Desc: "JSON 格式的数据对象，包含所有占位符的值", Required: true},
		}),
	}, nil
}

func (t *htmlInterpreterTool) InvokableRun(ctx context.Context, argumentsInJSON string, opt ...tool.Option) (string, error) {
	type req struct {
		TemplatePath string `json:"template_path"`
		Data         string `json:"data"`
	}
	var r req
	if err := json.Unmarshal([]byte(argumentsInJSON), &r); err != nil {
		return "", fmt.Errorf("parse arguments failed: %w", err)
	}

	if r.TemplatePath == "" {
		return "", fmt.Errorf("template_path is required")
	}
	if r.Data == "" {
		return "", fmt.Errorf("data is required")
	}

	// 解析 data JSON
	var data map[string]any
	if err := json.Unmarshal([]byte(r.Data), &data); err != nil {
		return "", fmt.Errorf("parse data JSON failed: %w", err)
	}

	// 读取模板文件
	templatePath := r.TemplatePath
	if !filepath.IsAbs(templatePath) {
		templatePath = filepath.Join(t.workDir, templatePath)
	}
	templateContent, err := os.ReadFile(templatePath)
	if err != nil {
		return "", fmt.Errorf("read template failed: %w", err)
	}

	// 注入数据到模板
	html := string(templateContent)

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

	return html, nil
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
