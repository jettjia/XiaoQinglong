package prompt

import (
	"sync"
	"time"
)

// CachedPrompt represents a cached static sections prompt
type CachedPrompt struct {
	StaticSections string
	ToolsHash     string
	BuiltAt       time.Time
}

// PromptCache caches static sections of prompts per agent
type PromptCache struct {
	mu    sync.RWMutex
	cache map[string]*CachedPrompt // agentID -> cached prompt
}

// NewPromptCache creates a new prompt cache
func NewPromptCache() *PromptCache {
	return &PromptCache{
		cache: make(map[string]*CachedPrompt),
	}
}

// Get retrieves cached static sections for an agent
// Returns (content, toolsHash, found)
func (c *PromptCache) Get(agentID string) (string, string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if entry, ok := c.cache[agentID]; ok {
		return entry.StaticSections, entry.ToolsHash, true
	}
	return "", "", false
}

// Set stores static sections for an agent
func (c *PromptCache) Set(agentID string, staticSections string, toolsHash string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache[agentID] = &CachedPrompt{
		StaticSections: staticSections,
		ToolsHash:      toolsHash,
		BuiltAt:       time.Now(),
	}
}

// ShouldRefresh checks if the cache should be refreshed based on tools hash
func (c *PromptCache) ShouldRefresh(agentID string, currentToolsHash string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if entry, ok := c.cache[agentID]; ok {
		return entry.ToolsHash != currentToolsHash
	}
	return true
}

// Invalidate removes cached entry for an agent
func (c *PromptCache) Invalidate(agentID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.cache, agentID)
}

// Clear removes all cached entries
func (c *PromptCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache = make(map[string]*CachedPrompt)
}

// Stats returns cache statistics
func (c *PromptCache) Stats() (int, time.Time) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var oldest time.Time
	for _, entry := range c.cache {
		if oldest.IsZero() || entry.BuiltAt.Before(oldest) {
			oldest = entry.BuiltAt
		}
	}
	return len(c.cache), oldest
}
