package main

import (
	"context"
	"os"
	"path/filepath"
	"sync"

	"github.com/cloudwego/eino/compose"
	"github.com/jettjia/XiaoQinglong/runner/pkg/logger"
)

// FileCheckPointStore 基于文件的检查点存储，支持持久化
type FileCheckPointStore struct {
	dir string
	mu  sync.RWMutex
}

func NewFileCheckPointStore(dir string) compose.CheckPointStore {
	// 确保目录存在
	os.MkdirAll(dir, 0755)
	return &FileCheckPointStore{dir: dir}
}

func (f *FileCheckPointStore) filePath(key string) string {
	return filepath.Join(f.dir, key+".checkpoint")
}

func (f *FileCheckPointStore) Set(ctx context.Context, key string, value []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	filePath := f.filePath(key)
	err := os.WriteFile(filePath, value, 0644)
	if err != nil {
		logger.GetRunnerLogger().Infof("[FileCheckPointStore] Failed to write checkpoint %s: %v", key, err)
		return err
	}
	logger.GetRunnerLogger().Infof("[FileCheckPointStore] Saved checkpoint %s (%d bytes)", key, len(value))
	return nil
}

func (f *FileCheckPointStore) Get(ctx context.Context, key string) ([]byte, bool, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	filePath := f.filePath(key)
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			logger.GetRunnerLogger().Infof("[FileCheckPointStore] Checkpoint %s not found", key)
			return nil, false, nil
		}
		logger.GetRunnerLogger().Infof("[FileCheckPointStore] Failed to read checkpoint %s: %v", key, err)
		return nil, false, err
	}
	logger.GetRunnerLogger().Infof("[FileCheckPointStore] Loaded checkpoint %s (%d bytes)", key, len(data))
	return data, true, nil
}

// InMemoryCheckPointStore 内存中的检查点存储（不持久化，仅用于单次运行）
type InMemoryCheckPointStore struct {
	mem map[string][]byte
}

func NewInMemoryCheckPointStore() compose.CheckPointStore {
	return &InMemoryCheckPointStore{
		mem: map[string][]byte{},
	}
}

func (i *InMemoryCheckPointStore) Set(ctx context.Context, key string, value []byte) error {
	i.mem[key] = value
	return nil
}

func (i *InMemoryCheckPointStore) Get(ctx context.Context, key string) ([]byte, bool, error) {
	v, ok := i.mem[key]
	return v, ok, nil
}
