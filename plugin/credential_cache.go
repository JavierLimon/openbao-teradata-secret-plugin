package teradata

import (
	"context"
	"sync"
	"time"

	"github.com/JavierLimon/openbao-teradata-secret-plugin/models"
	"github.com/openbao/openbao/sdk/v2/logical"
)

type credentialCache struct {
	mu         sync.RWMutex
	entries    map[string]*cacheEntry
	ttl        time.Duration
	maxSize    int
	hitCount   int64
	missCount  int64
	evictCount int64
	done       chan struct{}
}

type cacheEntry struct {
	credential *models.Credential
	expiresAt  time.Time
	createdAt  time.Time
}

func newCredentialCache(ttl time.Duration, maxSize int) *credentialCache {
	cache := &credentialCache{
		entries: make(map[string]*cacheEntry),
		ttl:     ttl,
		maxSize: maxSize,
		done:    make(chan struct{}),
	}
	go cache.backgroundCleanup()
	return cache
}

func (c *credentialCache) Get(username string) (*models.Credential, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.entries[username]
	if !exists {
		c.missCount++
		return nil, false
	}

	if time.Now().After(entry.expiresAt) {
		c.mu.RUnlock()
		c.Delete(username)
		c.mu.RLock()
		c.missCount++
		return nil, false
	}

	c.hitCount++
	return entry.credential, true
}

func (c *credentialCache) Set(username string, cred *models.Credential) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.entries) >= c.maxSize {
		c.evictOldest()
	}

	entry := &cacheEntry{
		credential: cred,
		expiresAt:  time.Now().Add(c.ttl),
		createdAt:  time.Now(),
	}
	c.entries[username] = entry
}

func (c *credentialCache) Delete(username string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, username)
}

func (c *credentialCache) DeleteByRole(roleName string) []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	deleted := make([]string, 0)
	for username, entry := range c.entries {
		if entry.credential.RoleName == roleName {
			delete(c.entries, username)
			deleted = append(deleted, username)
		}
	}
	return deleted
}

func (c *credentialCache) Invalidate(username string) {
	c.Delete(username)
}

func (c *credentialCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]*cacheEntry)
}

func (c *credentialCache) Stats() (hits, misses, evicts int64, size int) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.hitCount, c.missCount, c.evictCount, len(c.entries)
}

func (c *credentialCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time
	now := time.Now()

	for key, entry := range c.entries {
		if oldestTime.IsZero() || entry.createdAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.createdAt
		}
	}

	if oldestKey != "" && now.Sub(oldestTime) > c.ttl {
		delete(c.entries, oldestKey)
		c.evictCount++
	}
}

func (c *credentialCache) backgroundCleanup() {
	ticker := time.NewTicker(c.ttl / 2)
	defer ticker.Stop()

	for {
		select {
		case <-c.done:
			return
		case <-ticker.C:
			c.cleanupExpired()
		}
	}
}

func (c *credentialCache) Close() {
	close(c.done)
}

func (c *credentialCache) cleanupExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for username, entry := range c.entries {
		if now.After(entry.expiresAt) {
			delete(c.entries, username)
			c.evictCount++
		}
	}
}

func (c *credentialCache) warmCache(ctx context.Context, storage logical.Storage, usernames []string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, username := range usernames {
		if _, exists := c.entries[username]; !exists {
			entry, err := getCredential(ctx, storage, username)
			if err == nil && entry != nil {
				c.entries[username] = &cacheEntry{
					credential: entry,
					expiresAt:  time.Now().Add(c.ttl),
					createdAt:  time.Now(),
				}
			}
		}
	}
	return nil
}
