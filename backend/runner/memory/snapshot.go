package memory

import (
	"sync"
	"time"
)

// SnapshotMode 快照模式
type SnapshotMode int

const (
	SnapshotModeFrozen SnapshotMode = iota // 冻结快照（不变）
	SnapshotModeLive                        // 实时快照（跟随变化）
)

// Snapshot 快照
type Snapshot struct {
	Content   string
	CreatedAt time.Time
	Mode      SnapshotMode
}

// SnapshotManager 快照管理器
type SnapshotManager struct {
	mu       sync.RWMutex
	snapshots map[string]*Snapshot // key: sessionID/userID/agentID

	// 回调函数，当快照更新时调用
	onUpdate map[string]func(string) // key -> callback
}

// NewSnapshotManager 创建快照管理器
func NewSnapshotManager() *SnapshotManager {
	return &SnapshotManager{
		snapshots: make(map[string]*Snapshot),
		onUpdate:  make(map[string]func(string)),
	}
}

// Capture 捕获快照
func (m *SnapshotManager) Capture(key string, content string, mode SnapshotMode) *Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()

	snapshot := &Snapshot{
		Content:   content,
		CreatedAt: time.Now(),
		Mode:      mode,
	}
	m.snapshots[key] = snapshot

	return snapshot
}

// Get 获取快照
func (m *SnapshotManager) Get(key string) *Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.snapshots[key]
}

// GetContent 获取快照内容
func (m *SnapshotManager) GetContent(key string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if snap, ok := m.snapshots[key]; ok {
		return snap.Content
	}
	return ""
}

// Update 更新快照（仅在 live 模式下生效）
func (m *SnapshotManager) Update(key string, content string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	snap, ok := m.snapshots[key]
	if !ok {
		return
	}

	// frozen 模式不更新
	if snap.Mode == SnapshotModeFrozen {
		return
	}

	snap.Content = content

	// 触发回调
	if cb, ok := m.onUpdate[key]; ok {
		go cb(content)
	}
}

// SetUpdateCallback 设置更新回调
func (m *SnapshotManager) SetUpdateCallback(key string, cb func(string)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onUpdate[key] = cb
}

// Delete 删除快照
func (m *SnapshotManager) Delete(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.snapshots, key)
	delete(m.onUpdate, key)
}

// ListKeys 列出所有快照 key
func (m *SnapshotManager) ListKeys() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	keys := make([]string, 0, len(m.snapshots))
	for k := range m.snapshots {
		keys = append(keys, k)
	}
	return keys
}

// MemoryContext 记忆上下文（用于传递给 Runner）
type MemoryContext struct {
	SessionID    string
	UserID       string
	AgentID      string
	SessionSnap string
	UserSnap    string
	AgentSnap   string
}

// NewMemoryContext 创建记忆上下文
func NewMemoryContext(sessionID, userID, agentID string, store *MemStore) *MemoryContext {
	ctx := &MemoryContext{
		SessionID: sessionID,
		UserID:    userID,
		AgentID:   agentID,
	}

	if store != nil {
		ctx.SessionSnap = store.GetSessionSnapshot(sessionID)
		ctx.UserSnap = store.GetUserSnapshot(userID)
		ctx.AgentSnap = store.GetAgentSnapshot(agentID)
	}

	return ctx
}

// ToPromptBlock 转换为 prompt 块
func (m *MemoryContext) ToPromptBlock() string {
	var blocks []string

	if m.SessionSnap != "" {
		blocks = append(blocks, FormatMemoryBlock(m.SessionSnap, EntryTypeSession))
	}
	if m.UserSnap != "" {
		blocks = append(blocks, FormatMemoryBlock(m.UserSnap, EntryTypeUser))
	}
	if m.AgentSnap != "" {
		blocks = append(blocks, FormatMemoryBlock(m.AgentSnap, EntryTypeAgent))
	}

	if len(blocks) == 0 {
		return ""
	}

	result := blocks[0]
	for i := 1; i < len(blocks); i++ {
		result += "\n\n" + blocks[i]
	}

	return result
}
