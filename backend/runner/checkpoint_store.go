package main

import (
	"context"

	"github.com/cloudwego/eino/compose"
)

// InMemoryCheckPointStore 内存中的检查点存储
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