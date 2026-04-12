package subagent

import (
	"sync"
)

// IterationBudget 迭代预算管理器，用于在父子 agent 之间共享迭代次数
type IterationBudget struct {
	mu          sync.RWMutex
	remaining   int
	total       int
	refunded    int             // 已退还的迭代次数（如 execute_code 等操作）
	exemptTools map[string]bool // 不消耗预算的工具（如 read-only 操作）
}

// NewIterationBudget 创建迭代预算
func NewIterationBudget(total int) *IterationBudget {
	return &IterationBudget{
		remaining: total,
		total:     total,
		refunded:  0,
		exemptTools: map[string]bool{
			"Read":          true, // 读取文件不消耗预算
			"Glob":          true,
			"Grep":          true,
			"WebFetch":      true,
			"WebSearch":     true,
			"TaskGet":       true,
			"TaskList":      true,
			"Sleep":         true,
			"EnterPlanMode": true,
			"ExitPlanMode":  true,
		},
	}
}

// Consume 尝试消耗一次迭代，返回是否成功
// 如果返回 false，表示预算已耗尽
func (b *IterationBudget) Consume() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.remaining <= 0 {
		return false
	}
	b.remaining--
	return true
}

// ConsumeForTool 根据工具类型决定是否消耗预算
// 只读工具（如 Read, Grep, WebSearch）不消耗预算
// 返回是否消耗了预算
func (b *IterationBudget) ConsumeForTool(toolName string) bool {
	// 如果是豁免工具，不消耗预算
	if b.exemptTools[toolName] {
		return false
	}
	return b.Consume()
}

// Refund 退还一次迭代（用于某些操作如 execute_code）
func (b *IterationBudget) Refund() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.remaining++
	b.refunded++
}

// Remaining 返回剩余迭代次数
func (b *IterationBudget) Remaining() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.remaining
}

// Total 返回总迭代次数
func (b *IterationBudget) Total() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.total
}

// Refunded 返回已退还的迭代次数
func (b *IterationBudget) Refunded() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.refunded
}

// Used 返回已使用的迭代次数
func (b *IterationBudget) Used() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.total - b.remaining
}

// IsExhausted 检查预算是否已耗尽
func (b *IterationBudget) IsExhausted() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.remaining <= 0
}

// AddExemptTool 添加豁免工具（不消耗预算的工具）
func (b *IterationBudget) AddExemptTool(toolName string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.exemptTools[toolName] = true
}

// IsExempt 检查工具是否豁免
func (b *IterationBudget) IsExempt(toolName string) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.exemptTools[toolName]
}
