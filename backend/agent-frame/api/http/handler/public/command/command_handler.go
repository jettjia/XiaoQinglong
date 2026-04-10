package command

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jettjia/igo-pkg/pkg/xerror"
	"github.com/jettjia/igo-pkg/pkg/xresponse"

	agentDto "github.com/jettjia/xiaoqinglong/agent-frame/application/dto/agent"
	agentSvc "github.com/jettjia/xiaoqinglong/agent-frame/application/service/agent"
	kbDto "github.com/jettjia/xiaoqinglong/agent-frame/application/dto/knowledge_base"
	kbSvc "github.com/jettjia/xiaoqinglong/agent-frame/application/service/knowledge_base"
	modelDto "github.com/jettjia/xiaoqinglong/agent-frame/application/dto/model"
	modelSvc "github.com/jettjia/xiaoqinglong/agent-frame/application/service/model"
	"github.com/jettjia/xiaoqinglong/agent-frame/types/apierror"
)

// Handler Command 处理
type Handler struct {
	agentSvc *agentSvc.SysAgentService
	kbSvc    *kbSvc.SysKnowledgeBaseService
	modelSvc *modelSvc.SysModelService
}

// NewHandler NewHandler
func NewHandler() *Handler {
	return &Handler{
		agentSvc: agentSvc.NewSysAgentService(),
		kbSvc:    kbSvc.NewSysKnowledgeBaseService(),
		modelSvc: modelSvc.NewSysModelService(),
	}
}

// ExecuteReq 执行命令请求
type ExecuteReq struct {
	Command string `json:"command" binding:"required"`
}

// ExecuteRsp 执行命令响应
type ExecuteRsp struct {
	Success     bool   `json:"success"`
	Action      string `json:"action"`
	Result      any    `json:"result,omitempty"`
	NavigateTo  string `json:"navigate_to,omitempty"`
	Message     string `json:"message,omitempty"`
	Prefilled   any    `json:"prefilled,omitempty"`
	ShowGuidance bool  `json:"show_guidance,omitempty"`
}

// intentResult 意图识别结果
type intentResult struct {
	Intent      string         `json:"intent"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Data        map[string]any `json:"data"`
}

// Execute 执行命令
func (h *Handler) Execute(c *gin.Context) {
	var req ExecuteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		err = xerror.NewErrorOpt(apierror.BadRequestErr, xerror.WithCause(err.Error()))
		_ = c.Error(err)
		return
	}

	if req.Command == "" {
		err := xerror.NewErrorOpt(apierror.BadRequestErr, xerror.WithCause("command is required"))
		_ = c.Error(err)
		return
	}

	// 意图识别（调用 LLM API）
	ir, err := h.recognizeIntentWithLLM(c.Request.Context(), req.Command)
	if err != nil {
		err = xerror.NewErrorOpt(apierror.InternalServerErr, xerror.WithCause("意图识别失败: "+err.Error()))
		_ = c.Error(err)
		return
	}

	// 根据 intent 执行对应逻辑
	var rsp ExecuteRsp
	switch ir.Intent {
	case "create_agent":
		rsp = h.executeCreateAgent(c.Request.Context(), ir.Data)
	case "add_model":
		rsp = h.executeAddModel(ir.Data)
	case "show_inbox":
		rsp = h.executeShowInbox(ir.Data)
	case "config_kb":
		rsp = h.executeConfigKB(ir.Data)
	case "test_kb_recall":
		rsp = h.executeTestKBRecall(c.Request.Context(), ir.Data)
	case "install_skill":
		rsp = h.executeInstallSkill(ir.Data)
	default:
		rsp = h.executeUnknown(ir.Description)
	}

	xresponse.RspOk(c, http.StatusOK, rsp)
}

// recognizeIntentWithLLM 使用 LLM 进行意图识别
func (h *Handler) recognizeIntentWithLLM(ctx context.Context, command string) (*intentResult, error) {
	// 获取模型配置
	model, err := h.getDefaultModel(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取模型配置失败: %w", err)
	}

	// 构建 prompt
	prompt := fmt.Sprintf(`你是一个 AI Agent 平台的管理助手。根据用户的指令，识别管理员意图。

支持的意图：
1. create_agent: 创建新智能体。提取 name, description。
2. add_model: 添加模型配置。提取 name, provider, api_base(可选)。
3. show_inbox: 查看收件箱/任务。
4. config_kb: 配置知识库。提取 name, retrieval_url。
5. test_kb_recall: 测试知识库召回。提取 kb_name(可选), query。
6. install_skill: 安装技能（无法自动完成，返回引导）。
7. unknown: 无法识别。

用户指令: "%s"

返回 JSON 格式（只返回 JSON，不要其他内容）：
{
  "intent": "意图类型",
  "title": "简短标题",
  "description": "操作描述",
  "data": { 提取的参数 }
}`, command)

	// 调用 LLM API
	reqBody := map[string]any{
		"model": model.Name,
		"messages": []map[string]any{
			{"role": "user", "content": prompt},
		},
		"temperature": 0.3,
		"max_tokens": 500,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("构建请求失败: %w", err)
	}

	// 构建 API URL
	apiURL := model.BaseUrl
	if !strings.HasSuffix(apiURL, "/v1") {
		apiURL = strings.TrimSuffix(apiURL, "/") + "/v1"
	}
	apiURL += "/chat/completions"

	// 创建请求
	httpReq, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+model.ApiKey)

	// 发送请求
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("调用 LLM 失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("LLM 返回错误 %d: %s", resp.StatusCode, string(respBody))
	}

	// 解析响应
	var llmResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &llmResp); err != nil {
		return nil, fmt.Errorf("解析 LLM 响应失败: %w", err)
	}

	if len(llmResp.Choices) == 0 {
		return nil, fmt.Errorf("LLM 返回为空")
	}

	content := llmResp.Choices[0].Message.Content

	// 解析意图 JSON
	var ir intentResult
	if err := json.Unmarshal([]byte(content), &ir); err != nil {
		// 尝试清理 JSON
		cleaned := cleanJSON(content)
		if err2 := json.Unmarshal([]byte(cleaned), &ir); err2 != nil {
			return nil, fmt.Errorf("解析意图 JSON 失败: %w, content: %s", err2, content)
		}
	}

	return &ir, nil
}

// cleanJSON 清理 JSON 字符串
func cleanJSON(s string) string {
	s = strings.TrimSpace(s)
	// 去除 markdown 代码块
	if strings.HasPrefix(s, "```") {
		lines := strings.SplitN(s, "\n", 2)
		if len(lines) > 1 {
			s = lines[1]
		}
	}
	if strings.HasSuffix(s, "```") {
		s = s[:len(s)-3]
	}
	return strings.TrimSpace(s)
}

// getDefaultModel 获取默认模型
func (h *Handler) getDefaultModel(ctx context.Context) (*modelDto.FindSysModelRsp, error) {
	models, err := h.modelSvc.FindSysModelAll(ctx, &modelDto.FindSysModelAllReq{ModelType: "llm"})
	if err != nil {
		return nil, err
	}

	if len(models) == 0 {
		return nil, fmt.Errorf("未配置 LLM 模型")
	}

	// 优先选择启用的模型
	for _, m := range models {
		if m.Status == "enabled" || m.Status == "" {
			return m, nil
		}
	}

	// 没有启用模型，返回第一个
	return models[0], nil
}

// executeCreateAgent 执行创建 Agent
func (h *Handler) executeCreateAgent(ctx context.Context, data map[string]any) ExecuteRsp {
	name, _ := data["name"].(string)
	if name == "" {
		return ExecuteRsp{
			Success: false,
			Action:  "create_agent",
			Message: "Agent 名称不能为空",
		}
	}

	description, _ := data["description"].(string)
	if description == "" {
		description = "由魔法盒创建的 " + name
	}

	// 调用 agent service 创建
	agentReq := &agentDto.CreateSysAgentReq{
		Name:        name,
		Description: description,
		Icon:        "Bot",
		Enabled:     true,
	}

	agentRsp, err := h.agentSvc.CreateSysAgent(ctx, agentReq)
	if err != nil {
		return ExecuteRsp{
			Success: false,
			Action:  "create_agent",
			Message: "创建失败: " + err.Error(),
		}
	}

	return ExecuteRsp{
		Success:    true,
		Action:     "create_agent",
		Result:     map[string]any{"agent_id": agentRsp.Ulid},
		NavigateTo: "agents",
		Message:    "智能体 \"" + name + "\" 创建成功",
	}
}

// executeAddModel 执行添加模型
func (h *Handler) executeAddModel(data map[string]any) ExecuteRsp {
	name, _ := data["name"].(string)
	provider, _ := data["provider"].(string)

	prefilled := make(map[string]any)
	if name != "" {
		prefilled["name"] = name
	}
	if provider != "" {
		prefilled["provider"] = provider
	}

	return ExecuteRsp{
		Success:    true,
		Action:     "add_model",
		NavigateTo: "models",
		Prefilled:  prefilled,
		Message:    "请填写模型配置",
	}
}

// executeShowInbox 执行查看收件箱
func (h *Handler) executeShowInbox(data map[string]any) ExecuteRsp {
	return ExecuteRsp{
		Success:    true,
		Action:     "show_inbox",
		NavigateTo: "inbox",
		Message:    "",
	}
}

// executeConfigKB 执行配置知识库
func (h *Handler) executeConfigKB(data map[string]any) ExecuteRsp {
	name, _ := data["name"].(string)
	retrievalURL, _ := data["retrieval_url"].(string)

	prefilled := make(map[string]any)
	if name != "" {
		prefilled["name"] = name
	}
	if retrievalURL != "" {
		prefilled["retrieval_url"] = retrievalURL
	}

	return ExecuteRsp{
		Success:    true,
		Action:     "config_kb",
		NavigateTo: "knowledge",
		Prefilled:  prefilled,
		Message:    "请填写知识库配置",
	}
}

// executeTestKBRecall 执行知识库召回测试
func (h *Handler) executeTestKBRecall(ctx context.Context, data map[string]any) ExecuteRsp {
	kbName, _ := data["kb_name"].(string)
	query, _ := data["query"].(string)

	if query == "" {
		query = "测试查询"
	}

	// 先获取所有知识库，找到匹配的那个
	kbAll, err := h.kbSvc.FindSysKnowledgeBaseAll(ctx, &kbDto.FindSysKnowledgeBaseAllReq{})
	if err != nil {
		return ExecuteRsp{
			Success: false,
			Action:  "test_kb_recall",
			Message: "获取知识库列表失败: " + err.Error(),
		}
	}

	// 如果指定了 kb_name，查找匹配的知识库
	var targetKB *kbDto.FindSysKnowledgeBaseRsp
	if kbName != "" {
		for _, kb := range kbAll {
			if kb.Name == kbName {
				targetKB = kb
				break
			}
		}
	}

	// 如果没找到指定的知识库，使用第一个启用的知识库
	if targetKB == nil {
		for _, kb := range kbAll {
			if kb.Enabled {
				targetKB = kb
				break
			}
		}
	}

	if targetKB == nil {
		return ExecuteRsp{
			Success: false,
			Action:  "test_kb_recall",
			Message: "没有可用的知识库，请先配置知识库",
		}
	}

	// 执行召回测试
	topK := 5
	recallReq := &kbDto.RecallTestReq{
		Query: query,
		TopK:  topK,
	}

	results, err := h.kbSvc.RecallTest(ctx, targetKB.Ulid, recallReq)
	if err != nil {
		return ExecuteRsp{
			Success: false,
			Action:  "test_kb_recall",
			Message: "召回测试失败: " + err.Error(),
		}
	}

	// 转换结果
	resultList := make([]map[string]any, 0)
	for _, r := range results {
		resultList = append(resultList, map[string]any{
			"title":   r.Title,
			"content": r.Content,
			"score":   r.Score,
		})
	}

	return ExecuteRsp{
		Success: true,
		Action:  "test_kb_recall",
		Result: map[string]any{
			"kb_name": targetKB.Name,
			"kb_id":   targetKB.Ulid,
			"query":   query,
			"results": resultList,
		},
		Message: fmt.Sprintf("召回测试完成，找到 %d 条结果", len(results)),
	}
}

// executeInstallSkill 执行技能引导
func (h *Handler) executeInstallSkill(data map[string]any) ExecuteRsp {
	return ExecuteRsp{
		Success:     true,
		Action:      "install_skill",
		ShowGuidance: true,
		Message:     "技能安装需要更多配置",
	}
}

// executeUnknown 执行未知意图
func (h *Handler) executeUnknown(description string) ExecuteRsp {
	return ExecuteRsp{
		Success: false,
		Action:  "unknown",
		Message: description,
	}
}
