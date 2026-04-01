package contextcompressor

// Token 阈值常量
const (
	AutoCompactBufferTokens     = 13_000
	WarningThresholdBufferTokens = 20_000
	ErrorThresholdBufferTokens   = 20_000
	ManualCompactBufferTokens    = 3_000
)

// ContextWindowSizes 常见模型的上下文窗口大小
var ContextWindowSizes = map[string]int{
	"claude-sonnet-4-20250514":   200_000,
	"claude-opus-4-20250514":     200_000,
	"claude-3-5-sonnet-20241022": 200_000,
	"claude-3-5-haiku-20241022": 200_000,
	"claude-3-opus-20240229":    200_000,
	"claude-3-sonnet-20240229":  200_000,
	"claude-3-haiku-20240307":   200_000,
	"gpt-4o":                    128_000,
	"gpt-4-turbo":               128_000,
	"gpt-3.5-turbo":             16_385,
}

// GetContextWindowSize 获取模型的上下文窗口大小
func GetContextWindowSize(model string) int {
	if size, ok := ContextWindowSizes[model]; ok {
		return size
	}
	// 默认值
	return 150_000
}

// GetEffectiveContextWindowSize 获取有效的上下文窗口大小（减去摘要输出预留）
func GetEffectiveContextWindowSize(model string) int {
	contextWindow := GetContextWindowSize(model)
	reservedForSummary := DefaultMaxOutputTokens
	if reservedForSummary > contextWindow {
		reservedForSummary = contextWindow
	}
	return contextWindow - reservedForSummary
}

// GetAutoCompactThreshold 获取自动压缩阈值
func GetAutoCompactThreshold(model string) int {
	effectiveWindow := GetEffectiveContextWindowSize(model)
	return effectiveWindow - AutoCompactBufferTokens
}

// ShouldAutoCompact 判断是否应该触发自动压缩
func ShouldAutoCompact(messages []Message, model string, tokenizer Tokenizer) bool {
	tokens := tokenizer.EstimateMessages(messages)
	threshold := GetAutoCompactThreshold(model)
	return tokens >= threshold
}

// TokenWarningState Token 警告状态
type TokenWarningState struct {
	PercentLeft                 int  // 剩余百分比
	IsAboveWarningThreshold     bool // 是否超过警告阈值
	IsAboveErrorThreshold       bool // 是否超过错误阈值
	IsAboveAutoCompactThreshold bool // 是否超过自动压缩阈值
	IsAtBlockingLimit           bool // 是否达到阻塞限制
}

// CalculateTokenWarningState 计算 Token 警告状态
func CalculateTokenWarningState(tokenUsage int, model string, autoCompactEnabled bool) TokenWarningState {
	autoCompactThreshold := GetAutoCompactThreshold(model)
	effectiveWindow := GetEffectiveContextWindowSize(model)

	if !autoCompactEnabled {
		effectiveWindow = GetContextWindowSize(model)
	}

	percentLeft := 0
	if effectiveWindow > 0 {
		percentLeft = max(0, ((effectiveWindow-tokenUsage)*100)/effectiveWindow)
	}

	warningThreshold := effectiveWindow - WarningThresholdBufferTokens
	errorThreshold := effectiveWindow - ErrorThresholdBufferTokens
	blockingLimit := effectiveWindow - ManualCompactBufferTokens

	return TokenWarningState{
		PercentLeft:                 percentLeft,
		IsAboveWarningThreshold:     tokenUsage >= warningThreshold,
		IsAboveErrorThreshold:       tokenUsage >= errorThreshold,
		IsAboveAutoCompactThreshold: autoCompactEnabled && tokenUsage >= autoCompactThreshold,
		IsAtBlockingLimit:           tokenUsage >= blockingLimit,
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
