package memory

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/jettjia/XiaoQinglong/runner/pkg/logger"
	"github.com/jettjia/XiaoQinglong/runner/pkg/xqldir"
	"github.com/jettjia/XiaoQinglong/runner/types"
)

// EntryType 记忆条目类型
type EntryType string

const (
	EntryTypeSession EntryType = "session"
	EntryTypeUser    EntryType = "user"
	EntryTypeAgent   EntryType = "agent"
)

// MemoryEntry 记忆条目
type MemoryEntry struct {
	Type      EntryType `json:"type"`
	ID        string    `json:"id"`      // session_id / user_id / agent_id
	Key       string    `json:"key"`     // 记忆 key
	Content   string    `json:"content"` // 记忆内容
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// MemStore 记忆存储
type MemStore struct {
	mu      sync.RWMutex
	baseDir string

	// live state（内存中的当前状态）
	sessionEntries map[string][]MemoryEntry // sessionID -> entries
	userEntries    map[string][]MemoryEntry // userID -> entries
	agentEntries   map[string][]MemoryEntry // agentID -> entries

	// 冻结快照（会话开始时捕获，用于 system prompt）
	sessionSnapshots map[string]string // sessionID -> snapshot
	userSnapshots    map[string]string // userID -> snapshot
	agentSnapshots   map[string]string // agentID -> snapshot

	// 安全扫描正则
	injectionPatterns []*regexp.Regexp
}

// NewMemStore 创建记忆存储
func NewMemStore() *MemStore {
	store := &MemStore{
		baseDir:          xqldir.GetMemoryDir(),
		sessionEntries:   make(map[string][]MemoryEntry),
		userEntries:      make(map[string][]MemoryEntry),
		agentEntries:     make(map[string][]MemoryEntry),
		sessionSnapshots: make(map[string]string),
		userSnapshots:    make(map[string]string),
		agentSnapshots:   make(map[string]string),
	}

	// 初始化安全扫描模式
	store.injectionPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)ignore\s+(previous|all)\s+instructions`),
		regexp.MustCompile(`(?i)you\s+are\s+now\s+(a|an)`),
		regexp.MustCompile(`(?i)disregard\s+(previous|all)\s+(instructions?|rules?)`),
		regexp.MustCompile(`\$[A-Z_]+\s*=.*curl|wget`),
		regexp.MustCompile(`authorized_keys`),
		regexp.MustCompile(`(?i)\.ssh/`),
	}

	return store
}

// ============ 层级目录路径 ============

// sessionDir 返回 session 记忆目录
func (s *MemStore) sessionDir(sessionID string) string {
	return filepath.Join(s.baseDir, "sessions", sessionID)
}

// userDir 返回 user 记忆目录
func (s *MemStore) userDir(userID string) string {
	return filepath.Join(s.baseDir, "users", userID)
}

// agentDir 返回 agent 记忆目录
func (s *MemStore) agentDir(agentID string) string {
	return filepath.Join(s.baseDir, "agents", agentID)
}

// ============ 文件名 ============

const (
	memoryFile  = "MEMORY.md"
	contextFile = "CONTEXT.md"
	userFile    = "USER.md"
	skillsFile  = "SKILLS.md"
)

// ============ 初始化（加载快照）============

// InitializeSession 初始化 session 记忆，加载冻结快照
func (s *MemStore) InitializeSession(ctx context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	dir := s.sessionDir(sessionID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create session dir failed: %w", err)
	}

	// 加载已有记忆
	entries, err := s.loadEntriesFromDir(ctx, dir, EntryTypeSession, sessionID)
	if err != nil {
		return err
	}
	s.sessionEntries[sessionID] = entries

	// 捕获冻结快照
	s.sessionSnapshots[sessionID] = s.buildSnapshot(entries)

	return nil
}

// InitializeUser 初始化 user 记忆
func (s *MemStore) InitializeUser(ctx context.Context, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	dir := s.userDir(userID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create user dir failed: %w", err)
	}

	entries, err := s.loadEntriesFromDir(ctx, dir, EntryTypeUser, userID)
	if err != nil {
		return err
	}
	s.userEntries[userID] = entries

	s.userSnapshots[userID] = s.buildSnapshot(entries)

	return nil
}

// InitializeAgent 初始化 agent 记忆
func (s *MemStore) InitializeAgent(ctx context.Context, agentID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	dir := s.agentDir(agentID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create agent dir failed: %w", err)
	}

	entries, err := s.loadEntriesFromDir(ctx, dir, EntryTypeAgent, agentID)
	if err != nil {
		return err
	}
	s.agentEntries[agentID] = entries

	s.agentSnapshots[agentID] = s.buildSnapshot(entries)

	return nil
}

// InitializeAll 初始化所有层级的记忆
func (s *MemStore) InitializeAll(ctx context.Context, sessionID, userID, agentID string) error {
	if sessionID != "" {
		if err := s.InitializeSession(ctx, sessionID); err != nil {
			return err
		}
	}
	if userID != "" {
		if err := s.InitializeUser(ctx, userID); err != nil {
			return err
		}
	}
	if agentID != "" {
		if err := s.InitializeAgent(ctx, agentID); err != nil {
			return err
		}
	}
	return nil
}

// ============ 获取快照（用于 system prompt）============

// GetSessionSnapshot 获取 session 冻结快照
func (s *MemStore) GetSessionSnapshot(sessionID string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sessionSnapshots[sessionID]
}

// GetUserSnapshot 获取 user 冻结快照
func (s *MemStore) GetUserSnapshot(userID string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.userSnapshots[userID]
}

// GetAgentSnapshot 获取 agent 冻结快照
func (s *MemStore) GetAgentSnapshot(agentID string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.agentSnapshots[agentID]
}

// ============ 添加记忆 ============

// Add 添加记忆条目
func (s *MemStore) Add(ctx context.Context, entry MemoryEntry) error {
	// 安全扫描
	if s.scanContent(entry.Content) {
		return fmt.Errorf("memory content failed security scan")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var dir string
	var entries []MemoryEntry
	var snapshot string

	switch entry.Type {
	case EntryTypeSession:
		dir = s.sessionDir(entry.ID)
		entries = s.sessionEntries[entry.ID]
		snapshot = s.sessionSnapshots[entry.ID]
	case EntryTypeUser:
		dir = s.userDir(entry.ID)
		entries = s.userEntries[entry.ID]
		snapshot = s.userSnapshots[entry.ID]
	case EntryTypeAgent:
		dir = s.agentDir(entry.ID)
		entries = s.agentEntries[entry.ID]
		snapshot = s.agentSnapshots[entry.ID]
	default:
		return fmt.Errorf("unknown entry type: %s", entry.Type)
	}

	// 去重检查
	for _, e := range entries {
		if e.Key == entry.Key {
			return fmt.Errorf("duplicate key: %s", entry.Key)
		}
	}

	entry.CreatedAt = time.Now()
	entry.UpdatedAt = time.Now()
	entries = append(entries, entry)

	// 持久化到文件
	if err := s.saveToFile(ctx, dir, entry); err != nil {
		return err
	}

	// 更新内存状态和快照
	switch entry.Type {
	case EntryTypeSession:
		s.sessionEntries[entry.ID] = entries
		s.sessionSnapshots[entry.ID] = s.buildSnapshot(entries)
	case EntryTypeUser:
		s.userEntries[entry.ID] = entries
		s.userSnapshots[entry.ID] = s.buildSnapshot(entries)
	case EntryTypeAgent:
		s.agentEntries[entry.ID] = entries
		s.agentSnapshots[entry.ID] = s.buildSnapshot(entries)
	}

	// 静默忽略未使用的 snapshot 变量
	_ = snapshot

	return nil
}

// ============ 替换记忆 ============

// Replace 替换记忆条目
func (s *MemStore) Replace(ctx context.Context, entry MemoryEntry) error {
	if s.scanContent(entry.Content) {
		return fmt.Errorf("memory content failed security scan")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var dir string
	var entries []MemoryEntry

	switch entry.Type {
	case EntryTypeSession:
		dir = s.sessionDir(entry.ID)
		entries = s.sessionEntries[entry.ID]
	case EntryTypeUser:
		dir = s.userDir(entry.ID)
		entries = s.userEntries[entry.ID]
	case EntryTypeAgent:
		dir = s.agentDir(entry.ID)
		entries = s.agentEntries[entry.ID]
	default:
		return fmt.Errorf("unknown entry type: %s", entry.Type)
	}

	// 查找并替换
	found := false
	for i, e := range entries {
		if e.Key == entry.Key {
			entry.CreatedAt = e.CreatedAt
			entry.UpdatedAt = time.Now()
			entries[i] = entry
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("key not found: %s", entry.Key)
	}

	// 重新持久化
	if err := s.saveToFile(ctx, dir, entry); err != nil {
		return err
	}

	// 更新内存状态
	switch entry.Type {
	case EntryTypeSession:
		s.sessionEntries[entry.ID] = entries
		s.sessionSnapshots[entry.ID] = s.buildSnapshot(entries)
	case EntryTypeUser:
		s.userEntries[entry.ID] = entries
		s.userSnapshots[entry.ID] = s.buildSnapshot(entries)
	case EntryTypeAgent:
		s.agentEntries[entry.ID] = entries
		s.agentSnapshots[entry.ID] = s.buildSnapshot(entries)
	}

	return nil
}

// ============ 删除记忆 ============

// Remove 删除记忆条目
func (s *MemStore) Remove(ctx context.Context, entryType EntryType, id, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var dir string
	var entries []MemoryEntry

	switch entryType {
	case EntryTypeSession:
		dir = s.sessionDir(id)
		entries = s.sessionEntries[id]
	case EntryTypeUser:
		dir = s.userDir(id)
		entries = s.userEntries[id]
	case EntryTypeAgent:
		dir = s.agentDir(id)
		entries = s.agentEntries[id]
	default:
		return fmt.Errorf("unknown entry type: %s", entryType)
	}

	// 查找并删除
	found := false
	for i, e := range entries {
		if e.Key == key {
			entries = append(entries[:i], entries[i+1:]...)
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("key not found: %s", key)
	}

	// 删除文件
	filePath := filepath.Join(dir, memoryFile)
	if err := s.removeFromFile(ctx, filePath, key); err != nil {
		// 不返回错误，只是警告
	}

	// 更新内存状态和快照
	switch entryType {
	case EntryTypeSession:
		s.sessionEntries[id] = entries
		s.sessionSnapshots[id] = s.buildSnapshot(entries)
	case EntryTypeUser:
		s.userEntries[id] = entries
		s.userSnapshots[id] = s.buildSnapshot(entries)
	case EntryTypeAgent:
		s.agentEntries[id] = entries
		s.agentSnapshots[id] = s.buildSnapshot(entries)
	}

	return nil
}

// ============ 查询记忆 ============

// GetAll 获取所有记忆条目
func (s *MemStore) GetAll(entryType EntryType, id string) []MemoryEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	switch entryType {
	case EntryTypeSession:
		return s.sessionEntries[id]
	case EntryTypeUser:
		return s.userEntries[id]
	case EntryTypeAgent:
		return s.agentEntries[id]
	}
	return nil
}

// Search 搜索记忆
func (s *MemStore) Search(ctx context.Context, entryType EntryType, id string, keyword string) []MemoryEntry {
	entries := s.GetAll(entryType, id)
	var results []MemoryEntry
	for _, e := range entries {
		if strings.Contains(e.Content, keyword) || strings.Contains(e.Key, keyword) {
			results = append(results, e)
		}
	}
	return results
}

// ============ 文件操作 ============

// loadEntriesFromDir 从目录加载记忆条目
func (s *MemStore) loadEntriesFromDir(ctx context.Context, dir string, entryType EntryType, id string) ([]MemoryEntry, error) {
	var entries []MemoryEntry
	filePath := filepath.Join(dir, memoryFile)

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read memory file failed: %w", err)
	}

	// 解析 JSON 文件
	var memoryData struct {
		Entries []MemoryEntry `json:"entries"`
	}
	if err := json.Unmarshal(data, &memoryData); err != nil {
		// 兼容旧格式，尝试逐行解析
		entries = s.parseLegacyFormat(data)
	} else {
		entries = memoryData.Entries
	}

	return entries, nil
}

// parseLegacyFormat 解析旧格式（纯文本）
func (s *MemStore) parseLegacyFormat(data []byte) []MemoryEntry {
	var entries []MemoryEntry
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		entries = append(entries, MemoryEntry{
			Content:   line,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		})
	}
	return entries
}

// saveToFile 保存记忆到文件
func (s *MemStore) saveToFile(ctx context.Context, dir string, entry MemoryEntry) error {
	filePath := filepath.Join(dir, memoryFile)

	// 获取当前文件内容
	var currentEntries []MemoryEntry
	data, err := os.ReadFile(filePath)
	if err == nil {
		json.Unmarshal(data, &currentEntries)
	}

	// 替换或追加
	found := false
	for i, e := range currentEntries {
		if e.Key == entry.Key {
			currentEntries[i] = entry
			found = true
			break
		}
	}
	if !found {
		currentEntries = append(currentEntries, entry)
	}

	// 写入文件
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(map[string]interface{}{"entries": currentEntries}); err != nil {
		return fmt.Errorf("encode memory failed: %w", err)
	}

	// 原子写入（先写临时文件再 rename）
	tmpPath := filePath + ".tmp"
	if err := os.WriteFile(tmpPath, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("write memory file failed: %w", err)
	}
	if err := os.Rename(tmpPath, filePath); err != nil {
		return fmt.Errorf("rename memory file failed: %w", err)
	}

	return nil
}

// removeFromFile 从文件中删除记忆
func (s *MemStore) removeFromFile(ctx context.Context, filePath, key string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	var memoryData struct {
		Entries []MemoryEntry `json:"entries"`
	}
	if err := json.Unmarshal(data, &memoryData); err != nil {
		return err
	}

	// 过滤掉要删除的条目
	var newEntries []MemoryEntry
	for _, e := range memoryData.Entries {
		if e.Key != key {
			newEntries = append(newEntries, e)
		}
	}

	// 重新写入
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(map[string]interface{}{"entries": newEntries}); err != nil {
		return err
	}

	tmpPath := filePath + ".tmp"
	if err := os.WriteFile(tmpPath, buf.Bytes(), 0644); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, filePath); err != nil {
		return err
	}

	return nil
}

// ============ 快照构建 ============

// buildSnapshot 构建快照字符串
func (s *MemStore) buildSnapshot(entries []MemoryEntry) string {
	if len(entries) == 0 {
		return ""
	}

	var buf bytes.Buffer
	buf.WriteString("══════════════════════════════════════════════\n")
	buf.WriteString("MEMORY\n")
	buf.WriteString("══════════════════════════════════════════════\n")

	for _, e := range entries {
		buf.WriteString(e.Content)
		buf.WriteString("\n")
		buf.WriteString("§\n") // 分隔符
	}

	return buf.String()
}

// ============ 安全扫描 ============

// scanContent 扫描内容是否包含恶意模式
func (s *MemStore) scanContent(content string) bool {
	for _, pattern := range s.injectionPatterns {
		if pattern.MatchString(content) {
			return true
		}
	}
	return false
}

// ============ 工具方法 ============

// GetMemoryPath 获取记忆目录路径
func (s *MemStore) GetMemoryPath() string {
	return s.baseDir
}

// FormatMemoryBlock 格式化记忆块用于注入 system prompt
func FormatMemoryBlock(snapshot string, entryType EntryType) string {
	if snapshot == "" {
		return ""
	}

	header := "MEMORY"
	switch entryType {
	case EntryTypeSession:
		header = "SESSION MEMORY"
	case EntryTypeUser:
		header = "USER MEMORY"
	case EntryTypeAgent:
		header = "AGENT MEMORY"
	}

	return fmt.Sprintf(`══════════════════════════════════════════════
%s
══════════════════════════════════════════════
%s`, header, snapshot)
}

// SaveMemoriesFromTypes 保存记忆（兼容 types.MemoryEntry）
// sessionID: 会话ID
// userID: 用户ID（用于 user 类型记忆）
// memories: 记忆条目列表（来自 types.MemoryEntry）
func (s *MemStore) SaveMemoriesFromTypes(sessionID, userID string, memories []types.MemoryEntry) error {
	if len(memories) == 0 {
		return nil
	}

	// 确定 user ID（优先使用 userID，否则使用 sessionID）
	actualUserID := userID
	if actualUserID == "" {
		actualUserID = sessionID
	}

	// 根据记忆类型分别存储
	for _, m := range memories {
		entry := MemoryEntry{
			Key:       m.Name, // 使用 name 作为 key
			Content:   m.Content,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		// 根据类型确定存储层级
		switch m.Type {
		case "user":
			entry.Type = EntryTypeUser
			entry.ID = actualUserID // 使用 userID（跨会话持久化）
		case "feedback":
			entry.Type = EntryTypeSession
			entry.ID = sessionID
		case "project":
			entry.Type = EntryTypeSession
			entry.ID = sessionID
		case "reference":
			entry.Type = EntryTypeSession
			entry.ID = sessionID
		default:
			entry.Type = EntryTypeSession
			entry.ID = sessionID
		}

		if err := s.Add(context.Background(), entry); err != nil {
			logger.GetRunnerLogger().Infof("[MemStore] Save memory failed: %v (key=%s)", err, entry.Key)
			// 继续保存其他记忆，不中断
		}
	}

	return nil
}
