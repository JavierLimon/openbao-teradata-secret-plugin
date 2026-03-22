package teradata

import (
	"container/list"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/JavierLimon/openbao-teradata-secret-plugin/models"
	"github.com/openbao/openbao/sdk/v2/logical"
)

type queryResultCache struct {
	mu         sync.RWMutex
	entries    map[string]*queryCacheEntry
	ttl        time.Duration
	maxSize    int
	hitCount   int64
	missCount  int64
	evictCount int64
	done       chan struct{}
}

type queryCacheEntry struct {
	Key        string
	Value      interface{}
	ExpiresAt  time.Time
	CreatedAt  time.Time
	AccessTime time.Time
	Element    *list.Element
}

type queryCacheStats struct {
	Hits      int64
	Misses    int64
	Evictions int64
	Size      int
	MaxSize   int
	TTL       time.Duration
}

type QueryResult struct {
	Columns  []string               `json:"columns"`
	Rows     [][]interface{}        `json:"rows"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

const (
	DefaultQueryCacheTTL     = 5 * time.Minute
	DefaultQueryCacheMaxSize = 1000
)

func newQueryResultCache(ttl time.Duration, maxSize int) *queryResultCache {
	if ttl <= 0 {
		ttl = DefaultQueryCacheTTL
	}
	if maxSize <= 0 {
		maxSize = DefaultQueryCacheMaxSize
	}

	cache := &queryResultCache{
		entries: make(map[string]*queryCacheEntry),
		ttl:     ttl,
		maxSize: maxSize,
		done:    make(chan struct{}),
	}
	go cache.backgroundCleanup()
	return cache
}

func (c *queryResultCache) Get(key string) (interface{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, exists := c.entries[key]
	if !exists {
		c.missCount++
		return nil, false
	}

	if time.Now().After(entry.ExpiresAt) {
		c.deleteEntry(key, entry)
		c.missCount++
		return nil, false
	}

	entry.AccessTime = time.Now()
	c.hitCount++
	return entry.Value, true
}

func (c *queryResultCache) Set(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if entry, exists := c.entries[key]; exists {
		entry.Value = value
		entry.ExpiresAt = time.Now().Add(c.ttl)
		entry.AccessTime = time.Now()
		return
	}

	if len(c.entries) >= c.maxSize {
		c.evictLRU()
	}

	entry := &queryCacheEntry{
		Key:        key,
		Value:      value,
		ExpiresAt:  time.Now().Add(c.ttl),
		CreatedAt:  time.Now(),
		AccessTime: time.Now(),
	}
	c.entries[key] = entry
}

func (c *queryResultCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, exists := c.entries[key]
	if exists {
		c.deleteEntry(key, entry)
	}
}

func (c *queryResultCache) DeleteByPrefix(prefix string) []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	deleted := make([]string, 0)
	for key, entry := range c.entries {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			c.deleteEntry(key, entry)
			deleted = append(deleted, key)
		}
	}
	return deleted
}

func (c *queryResultCache) Invalidate(key string) {
	c.Delete(key)
}

func (c *queryResultCache) InvalidateByPrefix(prefix string) []string {
	return c.DeleteByPrefix(prefix)
}

func (c *queryResultCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*queryCacheEntry)
}

func (c *queryResultCache) Stats() queryCacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return queryCacheStats{
		Hits:      c.hitCount,
		Misses:    c.missCount,
		Evictions: c.evictCount,
		Size:      len(c.entries),
		MaxSize:   c.maxSize,
		TTL:       c.ttl,
	}
}

func (c *queryResultCache) deleteEntry(key string, entry *queryCacheEntry) {
	delete(c.entries, key)
	c.evictCount++
}

func (c *queryResultCache) evictLRU() {
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range c.entries {
		if oldestTime.IsZero() || entry.AccessTime.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.AccessTime
		}
	}

	if oldestKey != "" {
		delete(c.entries, oldestKey)
		c.evictCount++
	}
}

func (c *queryResultCache) backgroundCleanup() {
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

func (c *queryResultCache) Close() {
	select {
	case <-c.done:
		return
	default:
		close(c.done)
	}
}

func (c *queryResultCache) cleanupExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for key, entry := range c.entries {
		if now.After(entry.ExpiresAt) {
			delete(c.entries, key)
			c.evictCount++
		}
	}
}

type CacheKeyBuilder struct{}

func (ckb *CacheKeyBuilder) ConfigKey(region string) string {
	if region == "" {
		return "config:default"
	}
	return fmt.Sprintf("config:%s", region)
}

func (ckb *CacheKeyBuilder) RoleKey(name string) string {
	return fmt.Sprintf("role:%s", name)
}

func (ckb *CacheKeyBuilder) StatementKey(name string) string {
	return fmt.Sprintf("statement:%s", name)
}

func (ckb *CacheKeyBuilder) QueryKey(query string, args ...interface{}) string {
	h := sha256.New()
	h.Write([]byte(query))
	for _, arg := range args {
		h.Write([]byte(fmt.Sprintf("%v", arg)))
	}
	return fmt.Sprintf("query:%s", hex.EncodeToString(h.Sum(nil))[:16])
}

var cacheKeyBuilder = &CacheKeyBuilder{}

func (b *Backend) getCachedConfig(ctx context.Context, storage logical.Storage, region string) (*models.Config, error) {
	cache := b.getQueryCache()
	if cache == nil {
		return getConfigByName(ctx, storage, region)
	}

	key := cacheKeyBuilder.ConfigKey(region)
	if result, found := cache.Get(key); found {
		if cfg, ok := result.(*models.Config); ok {
			return cfg, nil
		}
	}

	cfg, err := getConfigByName(ctx, storage, region)
	if err != nil {
		return nil, err
	}

	if cfg != nil && cache != nil {
		cache.Set(key, cfg)
	}

	return cfg, nil
}

func (b *Backend) getCachedRole(ctx context.Context, storage logical.Storage, name string) (*models.Role, error) {
	cache := b.getQueryCache()
	if cache == nil {
		return getRole(ctx, storage, name)
	}

	key := cacheKeyBuilder.RoleKey(name)
	if result, found := cache.Get(key); found {
		if role, ok := result.(*models.Role); ok {
			return role, nil
		}
	}

	role, err := getRole(ctx, storage, name)
	if err != nil {
		return nil, err
	}

	if role != nil && cache != nil {
		cache.Set(key, role)
	}

	return role, nil
}

func (b *Backend) getCachedStatement(ctx context.Context, storage logical.Storage, name string) (*models.Statement, error) {
	cache := b.getQueryCache()
	if cache == nil {
		return getStatement(ctx, storage, name)
	}

	key := cacheKeyBuilder.StatementKey(name)
	if result, found := cache.Get(key); found {
		if stmt, ok := result.(*models.Statement); ok {
			return stmt, nil
		}
	}

	stmt, err := getStatement(ctx, storage, name)
	if err != nil {
		return nil, err
	}

	if stmt != nil && cache != nil {
		cache.Set(key, stmt)
	}

	return stmt, nil
}

func (b *Backend) getCachedQueryResult(ctx context.Context, query string, args ...interface{}) (*QueryResult, error) {
	cache := b.getQueryCache()
	if cache == nil {
		return nil, nil
	}

	key := cacheKeyBuilder.QueryKey(query, args...)
	if result, found := cache.Get(key); found {
		if qr, ok := result.(*QueryResult); ok {
			return qr, nil
		}
	}

	return nil, nil
}

func (b *Backend) cacheQueryResult(query string, result *QueryResult, args ...interface{}) {
	cache := b.getQueryCache()
	if cache == nil || result == nil {
		return
	}

	key := cacheKeyBuilder.QueryKey(query, args...)
	cache.Set(key, result)
}

func (b *Backend) invalidateConfigCache(region string) {
	cache := b.getQueryCache()
	if cache == nil {
		return
	}

	if region == "" {
		cache.Delete("config:default")
	} else {
		cache.Delete(cacheKeyBuilder.ConfigKey(region))
	}
}

func (b *Backend) invalidateRoleCache(name string) {
	cache := b.getQueryCache()
	if cache == nil {
		return
	}
	cache.Delete(cacheKeyBuilder.RoleKey(name))
}

func (b *Backend) invalidateRoleCacheByPrefix(rolePrefix string) {
	cache := b.getQueryCache()
	if cache == nil {
		return
	}
	cache.DeleteByPrefix("role:")
}

func (b *Backend) invalidateStatementCache(name string) {
	cache := b.getQueryCache()
	if cache == nil {
		return
	}
	cache.Delete(cacheKeyBuilder.StatementKey(name))
}

func (b *Backend) invalidateQueryCache(query string, args ...interface{}) {
	cache := b.getQueryCache()
	if cache == nil {
		return
	}

	key := cacheKeyBuilder.QueryKey(query, args...)
	cache.Delete(key)
}

func (b *Backend) invalidateAllQueryCache() {
	cache := b.getQueryCache()
	if cache == nil {
		return
	}
	cache.Clear()
}

func (b *Backend) getQueryCacheStats() queryCacheStats {
	cache := b.getQueryCache()
	if cache == nil {
		return queryCacheStats{}
	}
	return cache.Stats()
}
