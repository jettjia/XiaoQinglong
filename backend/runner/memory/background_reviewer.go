package memory

import (
	"context"
	"strings"
	"sync"

	"github.com/jettjia/XiaoQinglong/runner/pkg/logger"
	"github.com/jettjia/XiaoQinglong/runner/types"
)

// BackgroundReviewConfig 后台审查配置
type BackgroundReviewConfig struct {
	Enabled     bool // 是否启用后台审查
	MaxMemory   int  // 最多审查多少条记忆
	ModelConfig *types.ModelConfig
}

// DefaultBackgroundReviewConfig 返回默认配置
func DefaultBackgroundReviewConfig() *BackgroundReviewConfig {
	return &BackgroundReviewConfig{
		Enabled:   true,
		MaxMemory: 10,
	}
}

// BackgroundReviewCallback 审查完成后的回调
type BackgroundReviewCallback func(summary string)

// BackgroundReviewer 后台记忆/技能审查器
// _spawn_background_review 模式
// 在主响应发送后才在后台运行，不与主任务竞争模型注意力
type BackgroundReviewer struct {
	config       *BackgroundReviewConfig
	memStore     *MemStore
	callback     BackgroundReviewCallback
	mu           sync.RWMutex
	activeReview bool
}

// NewBackgroundReviewer 创建后台审查器
func NewBackgroundReviewer(config *BackgroundReviewConfig, memStore *MemStore) *BackgroundReviewer {
	if config == nil {
		config = DefaultBackgroundReviewConfig()
	}
	return &BackgroundReviewer{
		config:   config,
		memStore: memStore,
	}
}

// SetCallback 设置审查完成后的回调
func (r *BackgroundReviewer) SetCallback(callback BackgroundReviewCallback) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.callback = callback
}

// UpdateModelConfig 更新模型配置
func (r *BackgroundReviewer) UpdateModelConfig(modelConfig *types.ModelConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.config.ModelConfig = modelConfig
}

// ShouldReview 判断是否应该进行后台审查
func (r *BackgroundReviewer) ShouldReview() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.config.Enabled && !r.activeReview
}

// ReviewIfNeeded 检查是否需要触发后台审查
// 应该在主任务完成后调用
func (r *BackgroundReviewer) ReviewIfNeeded(ctx context.Context, userInput, assistantOutput string) {
	if !r.ShouldReview() {
		return
	}

	// 检查是否有关键词触发审查
	if !r.shouldTriggerReview(userInput, assistantOutput) {
		return
	}

	// 触发后台审查
	r.spawnBackgroundReview(ctx, userInput, assistantOutput)
}

// shouldTriggerReview 判断是否应该触发审查
// 只有当对话包含值得记忆/学习的内容时才触发
func (r *BackgroundReviewer) shouldTriggerReview(userInput, assistantOutput string) bool {
	// 如果输出很短，可能不需要审查
	if len(assistantOutput) < 100 {
		return false
	}

	// 如果用户明确要求记忆某些内容
	triggerKeywords := []string{
		"记住", "记住这个", "以后要", "下次记得",
		"save", "remember", "keep in mind",
		"don't forget", "never",
	}

	combined := userInput + assistantOutput
	for _, keyword := range triggerKeywords {
		if strings.Contains(strings.ToLower(combined), keyword) {
			return true
		}
	}

	// 如果输出很长或包含重要决策，也触发审查
	if len(assistantOutput) > 2000 {
		return true
	}

	return false
}

// spawnBackgroundReview 启动后台审查（_spawn_background_review）
// 使用独立 goroutine，不阻塞主流程
func (r *BackgroundReviewer) spawnBackgroundReview(ctx context.Context, userInput, assistantOutput string) {
	r.mu.Lock()
	if r.activeReview {
		r.mu.Unlock()
		return
	}
	r.activeReview = true
	callback := r.callback
	r.mu.Unlock()

	// 记录开始
	logger.GetRunnerLogger().Infof("[BackgroundReviewer] Starting background review")

	go func() {
		defer func() {
			r.mu.Lock()
			r.activeReview = false
			r.mu.Unlock()
		}()

		// 创建独立的 context，避免被主 context 取消
		reviewCtx := context.Background()

		// 执行记忆审查
		summary := r.doMemoryReview(reviewCtx, userInput, assistantOutput)

		// 如果有总结且有回调，调用回调
		if summary != "" && callback != nil {
			logger.GetRunnerLogger().Infof("[BackgroundReviewer] Review completed: %s", summary)
			callback(summary)
		}
	}()
}

// doMemoryReview 执行记忆审查
func (r *BackgroundReviewer) doMemoryReview(ctx context.Context, userInput, assistantOutput string) string {
	if r.config.ModelConfig == nil || r.config.ModelConfig.APIKey == "" {
		logger.GetRunnerLogger().Infof("[BackgroundReviewer] No model config, skipping review")
		return ""
	}

	// 创建记忆提取器
	extractor := NewMemoryExtractor(r.config.ModelConfig)

	// 提取记忆
	memories, err := extractor.ExtractMemories(ctx, userInput, assistantOutput)
	if err != nil {
		logger.GetRunnerLogger().Infof("[BackgroundReviewer] Memory extraction failed: %v", err)
		return ""
	}

	if len(memories) == 0 {
		logger.GetRunnerLogger().Infof("[BackgroundReviewer] No memories extracted")
		return ""
	}

	// 保存记忆到 store
	// 注意：sessionID 应该从调用者传入，这里使用空字符串
	sessionID := ""
	if r.memStore != nil {
		// 保存记忆
		if err := r.memStore.SaveMemoriesFromTypes(sessionID, "", memories); err != nil {
			logger.GetRunnerLogger().Infof("[BackgroundReviewer] Failed to save memories: %v", err)
			return ""
		}
	}

	// 生成总结
	var memoryTypes []string
	typeCount := make(map[string]int)
	for _, m := range memories {
		typeCount[m.Type]++
	}
	for t := range typeCount {
		memoryTypes = append(memoryTypes, t)
	}

	summary := "Saved " + strings.Join(memoryTypes, ", ") + " memories"
	logger.GetRunnerLogger().Infof("[BackgroundReviewer] %s", summary)
	return summary
}

// ReviewExistingMemories 审查并优化已有记忆
// 可以用于定期清理或合并相似记忆
func (r *BackgroundReviewer) ReviewExistingMemories(ctx context.Context, sessionID string) {
	if !r.ShouldReview() || r.memStore == nil {
		return
	}

	go func() {
		r.mu.Lock()
		r.activeReview = true
		r.mu.Unlock()

		defer func() {
			r.mu.Lock()
			r.activeReview = false
			r.mu.Unlock()
		}()

		logger.GetRunnerLogger().Infof("[BackgroundReviewer] Starting existing memories review for session: %s", sessionID)

		// 获取已有记忆
		entries := r.memStore.GetAll(EntryTypeSession, sessionID)
		if len(entries) == 0 {
			return
		}

		// 去重/合并相似记忆
		deduplicated := r.deduplicateMemoriesFromEntries(entries)

		// 如果有变化，更新
		if len(deduplicated) != len(entries) {
			logger.GetRunnerLogger().Infof("[BackgroundReviewer] Deduplicated %d -> %d memories",
				len(entries), len(deduplicated))
		}
	}()
}

// deduplicateMemoriesFromEntries 去重/合并相似记忆
func (r *BackgroundReviewer) deduplicateMemoriesFromEntries(entries []MemoryEntry) []MemoryEntry {
	if len(entries) <= 1 {
		return entries
	}

	// 简单去重：基于 Key 的组合
	seen := make(map[string]bool)
	var result []MemoryEntry

	for _, e := range entries {
		if !seen[e.Key] {
			seen[e.Key] = true
			result = append(result, e)
		}
	}

	return result
}

// GetActiveReviewStatus 获取当前审查状态
func (r *BackgroundReviewer) GetActiveReviewStatus() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.activeReview
}
