package plugins

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/jettjia/XiaoQinglong/runner/pkg/logger"
	"github.com/jettjia/XiaoQinglong/runner/types"
)

// ExecutionPlan 执行计划
type ExecutionPlan struct {
	Stages []ExecutionStage // 分阶段执行，每阶段内可并行
}

type ExecutionStage struct {
	Skills []SkillExecution // 该阶段内并行执行的 skill
}

type SkillExecution struct {
	Skill    types.Skill
	InputCtx map[string]any // 该 skill 的输入上下文 (从上游 skill 结果获取)
}

// SkillPlanner 技能规划器 - LLM 驱动
type SkillPlanner struct {
	skills      []types.Skill
	skillRunner *SkillRunner
	model       model.ToolCallingChatModel // 用于 LLM 规划的模型
}

// NewSkillPlanner 创建技能规划器
func NewSkillPlanner(skills []types.Skill, skillRunner *SkillRunner, model model.ToolCallingChatModel) *SkillPlanner {
	return &SkillPlanner{
		skills:      skills,
		skillRunner: skillRunner,
		model:       model,
	}
}

// PlanWithLLM 使用 LLM 分析上下文，自主决定需要执行哪些 skill 以及顺序
func (p *SkillPlanner) PlanWithLLM(ctx context.Context, userMessage string, contextData map[string]any) (*ExecutionPlan, error) {
	// 1. 构建技能描述供 LLM 参考
	skillDescriptions := p.buildSkillDescriptions()

	// 2. LLM 分析应该执行哪些 skill
	planResult, err := p.analyzeWithLLM(ctx, userMessage, contextData, skillDescriptions)
	if err != nil {
		return nil, fmt.Errorf("LLM analysis failed: %w", err)
	}

	// 3. 解析 LLM 返回的执行计划
	executionPlan, err := p.parseExecutionPlan(planResult, contextData)
	if err != nil {
		return nil, fmt.Errorf("parse execution plan failed: %w", err)
	}

	return executionPlan, nil
}

// buildSkillDescriptions 构建技能描述供 LLM 参考
func (p *SkillPlanner) buildSkillDescriptions() string {
	var sb strings.Builder
	sb.WriteString("可用技能列表:\n\n")
	for _, skill := range p.skills {
		sb.WriteString(fmt.Sprintf("## %s (%s)\n", skill.Name, skill.ID))
		sb.WriteString(fmt.Sprintf("描述: %s\n", skill.Description))
		if len(skill.Inputs) > 0 {
			sb.WriteString(fmt.Sprintf("需要输入: %s\n", strings.Join(skill.Inputs, ", ")))
		}
		if len(skill.Outputs) > 0 {
			sb.WriteString(fmt.Sprintf("产出输出: %s\n", strings.Join(skill.Outputs, ", ")))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// analyzeWithLLM 调用 LLM 分析应该执行哪些 skill
func (p *SkillPlanner) analyzeWithLLM(ctx context.Context, userMessage string, contextData map[string]any, skillDescriptions string) (string, error) {
	if p.model == nil {
		// 如果没有模型，使用默认规划
		return p.defaultPlanning(ctx, userMessage, contextData, skillDescriptions)
	}

	prompt := fmt.Sprintf(`你是一个任务规划助手。根据用户需求和上下文，决定需要执行哪些技能以及执行顺序。

用户需求: %s

当前上下文: %v

%s

请分析并返回 JSON 格式的执行计划:
{
  "needed_skills": ["skill_id1", "skill_id2"],  // 需要执行的 skill ID 列表（按执行顺序）
  "reasoning": "分析理由"  // 为什么需要这些 skill
}

只返回 JSON，不要其他内容。`, userMessage, contextData, skillDescriptions)

	messages := []adk.Message{
		schema.SystemMessage("你是一个任务规划助手，擅长分析用户需求并规划执行步骤。"),
		schema.UserMessage(prompt),
	}

	resp, err := p.model.Generate(ctx, messages)
	if err != nil {
		return "", err
	}

	return resp.Content, nil
}

// defaultPlanning 默认规划（当没有 LLM 时）
func (p *SkillPlanner) defaultPlanning(ctx context.Context, userMessage string, contextData map[string]any, skillDescriptions string) (string, error) {
	// 简单关键词匹配
	var neededSkillIDs []string
	userLower := strings.ToLower(userMessage)

	for _, skill := range p.skills {
		// 检查 skill 名称或描述是否匹配用户消息
		if strings.Contains(userLower, strings.ToLower(skill.Name)) ||
			strings.Contains(userLower, strings.ToLower(skill.ID)) {
			neededSkillIDs = append(neededSkillIDs, skill.ID)
		}
	}

	// 如果没有匹配，检查触发词
	if len(neededSkillIDs) == 0 {
		for _, skill := range p.skills {
			if skill.Trigger != "" && strings.Contains(userLower, strings.ToLower(skill.Trigger)) {
				neededSkillIDs = append(neededSkillIDs, skill.ID)
			}
		}
	}

	// 构建简单的 JSON 结果
	var sb strings.Builder
	sb.WriteString(`{"needed_skills":[`)
	for i, id := range neededSkillIDs {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(`"` + id + `"`)
	}
	sb.WriteString(`],"reasoning":"基于关键词匹配"}`)

	return sb.String(), nil
}

// parseExecutionPlan 解析 LLM 返回的执行计划
func (p *SkillPlanner) parseExecutionPlan(llmResult string, contextData map[string]any) (*ExecutionPlan, error) {
	// 简单解析 JSON (实际生产环境应该用 json.Unmarshal)
	neededSkillIDs, err := p.extractSkillIDs(llmResult)
	if err != nil {
		return nil, err
	}

	if len(neededSkillIDs) == 0 {
		return &ExecutionPlan{Stages: []ExecutionStage{}}, nil
	}

	// 构建 skill map 方便查找
	skillMap := make(map[string]types.Skill)
	for _, skill := range p.skills {
		skillMap[skill.ID] = skill
	}

	// 分析依赖关系，构建执行阶段
	stages := p.buildExecutionStages(neededSkillIDs, skillMap, contextData)

	return &ExecutionPlan{Stages: stages}, nil
}

// extractSkillIDs 从 LLM 返回中提取 skill ID 列表
func (p *SkillPlanner) extractSkillIDs(llmResult string) ([]string, error) {
	// 简单的字符串解析 "needed_skills":["skill1","skill2"]
	var ids []string
	start := strings.Index(llmResult, `"needed_skills"`)
	if start == -1 {
		return ids, nil
	}

	bracketStart := strings.Index(llmResult[start:], "[")
	bracketEnd := strings.Index(llmResult[start:], "]")
	if bracketStart == -1 || bracketEnd == -1 {
		return ids, nil
	}

	arrayStr := llmResult[start+bracketStart : start+bracketEnd+1]
	// 提取 "xxx" 格式的 ID
	parts := strings.Split(arrayStr, ",")
	for _, part := range parts {
		quoteStart := strings.Index(part, `"`)
		quoteEnd := strings.LastIndex(part, `"`)
		if quoteStart != -1 && quoteEnd != -1 && quoteEnd > quoteStart {
			id := part[quoteStart+1 : quoteEnd]
			ids = append(ids, id)
		}
	}

	return ids, nil
}

// buildExecutionStages 构建执行阶段，同一层可并行
func (p *SkillPlanner) buildExecutionStages(skillIDs []string, skillMap map[string]types.Skill, contextData map[string]any) []ExecutionStage {
	// 分析每个 skill 的输入是否可以被当前上下文满足
	var stages []ExecutionStage
	processed := make(map[string]bool)

	for len(processed) < len(skillIDs) {
		var currentStage []SkillExecution

		for _, skillID := range skillIDs {
			if processed[skillID] {
				continue
			}

			skill, ok := skillMap[skillID]
			if !ok {
				continue
			}

			// 检查依赖是否已满足
			if p.canExecute(skill, contextData, processed) {
				// 收集该 skill 的输入
				inputCtx := p.collectInputs(skill, contextData, processed)
				currentStage = append(currentStage, SkillExecution{
					Skill:    skill,
					InputCtx: inputCtx,
				})
				processed[skillID] = true
			}
		}

		if len(currentStage) == 0 {
			// 没有可执行的 skill，可能有循环依赖
			break
		}

		stages = append(stages, ExecutionStage{Skills: currentStage})
	}

	return stages
}

// canExecute 检查 skill 的依赖是否已满足
func (p *SkillPlanner) canExecute(skill types.Skill, contextData map[string]any, processed map[string]bool) bool {
	// 检查所有输入是否可满足
	for _, input := range skill.Inputs {
		// 检查是否在全局上下文中
		if _, ok := contextData[input]; ok {
			continue
		}
		// 检查是否由已处理的 skill 产出
		found := false
		for processedID := range processed {
			procSkill := p.getSkillByID(processedID)
			if procSkill != nil {
				for _, output := range procSkill.Outputs {
					if output == input {
						found = true
						break
					}
				}
			}
			if found {
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// collectInputs 收集 skill 执行所需的输入
func (p *SkillPlanner) collectInputs(skill types.Skill, contextData map[string]any, processed map[string]bool) map[string]any {
	inputCtx := make(map[string]any)

	for _, input := range skill.Inputs {
		// 先从全局上下文获取
		if v, ok := contextData[input]; ok {
			inputCtx[input] = v
			continue
		}
		// 从已处理的 skill 结果获取
		for processedID := range processed {
			procSkill := p.getSkillByID(processedID)
			if procSkill != nil {
				key := processedID + "_result"
				if v, ok := contextData[key]; ok {
					inputCtx[input] = v
					break
				}
			}
		}
	}

	return inputCtx
}

// getSkillByID 根据 ID 获取 skill
func (p *SkillPlanner) getSkillByID(id string) *types.Skill {
	for _, skill := range p.skills {
		if skill.ID == id {
			return &skill
		}
	}
	return nil
}

// Execute 执行计划
func (p *SkillPlanner) Execute(ctx context.Context, plan *ExecutionPlan) (map[string]any, error) {
	results := make(map[string]any)

	for stageIdx, stage := range plan.Stages {
		log.Printf("[SkillPlanner] Executing stage %d with %d skills (parallel)", stageIdx, len(stage.Skills))

		// 并行执行同阶段的 skill
		resultsCh := make(chan struct {
			skillID string
			result  any
			err     error
		}, len(stage.Skills))

		for _, exec := range stage.Skills {
			go func(skill types.Skill, inputCtx map[string]any) {
				result, err := p.executeSkill(ctx, skill, inputCtx)
				resultsCh <- struct {
					skillID string
					result  any
					err     error
				}{skill.ID, result, err}
			}(exec.Skill, exec.InputCtx)
		}

		// 等待所有 skill 完成
		for i := 0; i < len(stage.Skills); i++ {
			select {
			case res := <-resultsCh:
				if res.err != nil {
					logger.Errorf("[SkillPlanner] Skill %s failed: %v", res.skillID, res.err)
					results[res.skillID+"_error"] = res.err.Error()
				} else {
					logger.Infof("[SkillPlanner] Skill %s completed", res.skillID)
					results[res.skillID+"_result"] = res.result
				}
			case <-ctx.Done():
				return results, ctx.Err()
			}
		}
	}

	return results, nil
}

// executeSkill 执行单个 skill
func (p *SkillPlanner) executeSkill(ctx context.Context, skill types.Skill, inputCtx map[string]any) (any, error) {
	if p.skillRunner == nil {
		return nil, fmt.Errorf("skill runner not initialized")
	}

	start := time.Now()

	// 执行 skill (调用 SkillRunner.RunSkill)
	result, err := p.skillRunner.RunSkill(ctx, skill.ID, inputCtx, "")

	latency := time.Since(start).Milliseconds()
	logger.Infof("[SkillPlanner] Skill %s executed in %dms", skill.ID, latency)

	if err != nil {
		return nil, fmt.Errorf("skill %s execution failed: %w", skill.ID, err)
	}

	return result, nil
}
