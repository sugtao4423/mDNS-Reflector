package reflector

import (
	"context"
	"hash/fnv"
	"sync"
	"time"
)

const (
	dedupWindow     = 1 * time.Second
	cleanupInterval = 10 * time.Second
)

type dedupCache struct {
	mu      sync.Mutex
	entries map[uint64]time.Time
}

func newDedupCache() *dedupCache {
	return &dedupCache{
		entries: make(map[uint64]time.Time),
	}
}

func (c *dedupCache) isDuplicate(srcIface string, packet []byte) bool {
	h := fnv.New64a()
	h.Write([]byte(srcIface))
	h.Write(packet)
	key := h.Sum64()

	now := time.Now()

	c.mu.Lock()
	defer c.mu.Unlock()

	if expireAt, ok := c.entries[key]; ok && now.Before(expireAt) {
		return true
	}

	c.entries[key] = now.Add(dedupWindow)
	return false
}

func (c *dedupCache) cleanup() {
	now := time.Now()

	c.mu.Lock()
	defer c.mu.Unlock()

	for key, expireAt := range c.entries {
		if now.After(expireAt) {
			delete(c.entries, key)
		}
	}
}

func (c *dedupCache) runCleanup(ctx context.Context) {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.cleanup()
		}
	}
}
