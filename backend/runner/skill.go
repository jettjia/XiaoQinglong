package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
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
)

// ========== Skill Runner ==========

// SkillRunner runs skills progressively with sliding-window approach
type SkillRunner struct {
	skills     map[string]Skill
	skillsDir  string // 存储所有 skill 脚本的目录
	sandboxCfg *SandboxConfig
	model      model.ToolCallingChatModel // 用于执行 skill 的模型
	configMgr  *SkillConfigManager        // skill 配置管理器

	// session 管理：多步骤 skill 执行时共享工作目录
	// key: sessionID, value: sessionWorkDir
	sessions map[string]string

	// CurrentSessionID 由 dispatcher 设置，标识当前请求的 session
	CurrentSessionID string
}

// NewSkillRunner creates a new skill runner
func NewSkillRunner(skills []Skill, skillsDir string, sandboxCfg *SandboxConfig, model model.ToolCallingChatModel, configMgr *SkillConfigManager) *SkillRunner {
	skillMap := make(map[string]Skill)
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

	log.Printf("[Skill] Running skill: %s, input: %v, sessionID: %s", name, input, sessionID)

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
	hasScript := skill.FilePath != "" && skill.EntryScript != ""

	if hasScript && r.sandboxCfg != nil && r.sandboxCfg.Enabled {
		// 使用沙箱执行 skill
		return r.runSkillWithSandbox(ctx, skill, inputStr, sessionID)
	}

	// 无沙箱时，使用简单的模型调用方式执行 skill
	return r.runSkillSimple(ctx, skill, inputStr)
}

// runSkillWithSandbox 在沙箱中执行 skill（滑动窗口模式）
// sessionID 用于标识同一个请求中的多次 skill 调用，实现工作目录共享
func (r *SkillRunner) runSkillWithSandbox(ctx context.Context, skill Skill, input string, sessionID string) (string, error) {
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
		log.Printf("[Skill] Reusing session workdir: %s for session: %s", tmpDir, sessionID)
	} else {
		// 首次创建 session 工作目录
		tmpDir, err = os.MkdirTemp("", "skill-session-*")
		if err != nil {
			return "", fmt.Errorf("create session dir failed: %w", err)
		}
		r.sessions[sessionID] = tmpDir
		log.Printf("[Skill] Created new session workdir: %s for session: %s", tmpDir, sessionID)
	}

	// 4. 复制 skill 文件到工作目录
	if err := r.copySkillFiles(skillDir, tmpDir); err != nil {
		return "", fmt.Errorf("copy skill files failed: %w", err)
	}

	// 5. 创建沙箱工具：list_skill_files, read_skill_file, exec_skill_command
	tools := r.buildSkillSandboxTools(tmpDir, skill, input)

	// 5. 构建执行 instruction
	instruction := r.buildSkillInstruction(skill, input)

	// 6. 使用 adk agent 渐进式执行
	return r.runSkillWithAgent(ctx, instruction, tools)
}

// runSkillWithAgent 使用 adk agent 渐进式执行 skill
func (r *SkillRunner) runSkillWithAgent(ctx context.Context, instruction string, tools []tool.BaseTool) (string, error) {
	if r.model == nil {
		return "", fmt.Errorf("model not configured for skill execution")
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

	if lastContent == "" {
		return "", fmt.Errorf("skill execution returned no content")
	}

	return lastContent, nil
}

// runSkillSimple 简单模式执行 skill（无沙箱）
func (r *SkillRunner) runSkillSimple(ctx context.Context, skill Skill, input string) (string, error) {
	// 简单返回 skill instruction 作为执行指引
	return fmt.Sprintf("Skill: %s\nDescription: %s\nInstruction: %s\nInput: %s",
		skill.Name, skill.Description, skill.Instruction, input), nil
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
func (r *SkillRunner) buildSkillSandboxTools(workDir string, skill Skill, skillInput string) []tool.BaseTool {
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

	return []tool.BaseTool{listTool, readTool, execTool}
}

// buildSkillInstruction 构建 skill 执行 instruction
func (r *SkillRunner) buildSkillInstruction(skill Skill, input string) string {
	return fmt.Sprintf(`你是一个 Skill 执行助手。
当前 Skill 名称: %s
输入: %s

执行步骤：
1. 首先调用 read_skill_file 读取 SKILL.md，这是最重要的入口文件
2. 仔细阅读 SKILL.md 中的 Quick Reference 表，它会告诉你该读取哪个文件
3. 按照 SKILL.md 的指引，读取对应的文档
4. 根据文档指引，调用 exec_skill_command 执行所需命令
5. 返回执行结果

重要规则：
- 必须先读取 SKILL.md，不要先调用 list_skill_files
- 遵循 SKILL.md 中 Quick Reference 的指引去执行任务
- 调用 exec_skill_command 执行实际操作
- 工作目录为 %s`, skill.Name, input, r.skillsDir)
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
	sandboxCfg *SandboxConfig
	dockerBin  string
	skillName  string              // skill 名称，用于获取配置
	configMgr  *SkillConfigManager // 配置管理器
	skillInput string              // skill 输入参数，会传递给 SKILL_INPUT 环境变量
}

func (t *skillExecCommandTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "exec_skill_command",
		Desc: "在沙箱中执行 bash 命令",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"command": {Type: schema.String, Desc: "要执行的命令", Required: true},
		}),
	}, nil
}

func (t *skillExecCommandTool) InvokableRun(ctx context.Context, argumentsInJSON string, opt ...tool.Option) (string, error) {
	type req struct {
		Command string `json:"command"`
	}
	var r req
	if err := json.Unmarshal([]byte(argumentsInJSON), &r); err != nil {
		return "", err
	}

	command := strings.TrimSpace(r.Command)
	if command == "" {
		return "", fmt.Errorf("command is required")
	}

	// 替换 baseDirectory 占位符
	command = strings.ReplaceAll(command, "{{.BaseDirectory}}", t.baseDir)

	// 使用沙箱执行命令
	if t.sandboxCfg != nil && t.sandboxCfg.Enabled {
		return t.execInSandbox(ctx, command)
	}

	// 本地执行
	workDir := t.workDir
	if t.baseDir != "" {
		workDir = filepath.Join(workDir, t.baseDir)
	}

	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = workDir

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

	args = append(args, "-v", fmt.Sprintf("%s:%s", t.workDir, workdir))
	args = append(args, "--entrypoint", "", image, "sh", "-c", command)

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
		// 截断过长的描述
		if len([]rune(desc)) > 50 {
			desc = string([]rune(desc)[:50]) + "..."
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
