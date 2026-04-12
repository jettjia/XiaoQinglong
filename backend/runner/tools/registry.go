package tools

import (
	"context"
	"fmt"
	"sync"

	"github.com/cloudwego/eino/schema"
)

// ToolMeta 工具元信息
type ToolMeta struct {
	Name           string
	Desc           string
	IsReadOnly     bool                              // 是否只读工具，可并行执行
	MaxResultChars int                               // 最大结果字符数，超过则写入临时文件
	DefaultRisk    string                            // 默认风险级别: low, medium, high
	Creator        func(basePath string) interface{} // 创建工具实例的函数
}

// Registry 工具注册中心
type Registry struct {
	mu     sync.RWMutex
	tools  map[string]ToolMeta
	locker sync.Mutex
}

// 全局注册中心
var GlobalRegistry = NewRegistry()

// NewRegistry 创建新的注册中心
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]ToolMeta),
	}
}

// Register 注册工具
func (r *Registry) Register(meta ToolMeta) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[meta.Name] = meta
}

// Get 获取工具元信息
func (r *Registry) Get(name string) (ToolMeta, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	meta, ok := r.tools[name]
	return meta, ok
}

// List 返回所有已注册的工具名称
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}

// ListReadOnly 返回所有只读工具名称（可用于并行执行）
func (r *Registry) ListReadOnly() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var names []string
	for name, meta := range r.tools {
		if meta.IsReadOnly {
			names = append(names, name)
		}
	}
	return names
}

// CreateTool 根据名称创建工具实例
func (r *Registry) CreateTool(name string, basePath string) (BaseTool, error) {
	r.mu.RLock()
	meta, ok := r.tools[name]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("tool %s not found in registry", name)
	}

	creator := meta.Creator
	if creator == nil {
		return nil, fmt.Errorf("tool %s has no creator", name)
	}

	result := creator(basePath)
	if result == nil {
		return nil, fmt.Errorf("tool %s creator returned nil", name)
	}

	// 类型断言
	if tool, ok := result.(BaseTool); ok {
		return tool, nil
	}

	return nil, fmt.Errorf("tool %s creator did not return BaseTool", name)
}

// GetToolInfo 获取工具的 schema.ToolInfo
func (r *Registry) GetToolInfo(name string, basePath string) (*schema.ToolInfo, error) {
	tool, err := r.CreateTool(name, basePath)
	if err != nil {
		return nil, err
	}
	return tool.Info(context.Background())
}

// IsReadOnly 判断工具是否为只读
func (r *Registry) IsReadOnly(name string) bool {
	meta, ok := r.Get(name)
	if !ok {
		return false
	}
	return meta.IsReadOnly
}

// GetMaxResultChars 获取工具的最大结果字符数限制
func (r *Registry) GetMaxResultChars(name string) int {
	meta, ok := r.Get(name)
	if !ok {
		return 0 // 0 表示无限制
	}
	return meta.MaxResultChars
}

// GetDefaultRisk 获取工具的默认风险级别
func (r *Registry) GetDefaultRisk(name string) string {
	meta, ok := r.Get(name)
	if !ok {
		return "medium"
	}
	return meta.DefaultRisk
}

// Locker 返回注册中心的锁，用于外部同步
func (r *Registry) Locker() *sync.Mutex {
	return &r.locker
}
