package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/allegro/bigcache/v3"
)

// CacheManager implements multi-level caching
type CacheManager struct {
	l1Cache *redis.Client  // Redis for hot data
	l2Cache *bigcache.BigCache  // In-memory for very hot data
	config  *CacheConfig
}

// CacheConfig holds cache configuration
type CacheConfig struct {
	L1TTL     time.Duration
	L2TTL     time.Duration
	MaxSize   int
	L2MaxSize int
}

// DefaultCacheConfig returns sensible default cache configuration
func DefaultCacheConfig() *CacheConfig {
	return &CacheConfig{
		L1TTL:     15 * time.Minute,
		L2TTL:     5 * time.Minute,
		MaxSize:   1000,
		L2MaxSize: 100 * 1024 * 1024, // 100MB
	}
}

// NewCacheManager creates a new multi-level cache manager
func NewCacheManager(redisClient *redis.Client, config *CacheConfig) (*CacheManager, error) {
	if config == nil {
		config = DefaultCacheConfig()
	}

	l2Cache, err := bigcache.NewBigCache(bigcache.DefaultConfig(config.L2TTL))
	if err != nil {
		return nil, fmt.Errorf("failed to create L2 cache: %w", err)
	}

	return &CacheManager{
		l1Cache: redisClient,
		l2Cache: l2Cache,
		config:  config,
	}, nil
}

// Get retrieves a value from the cache
func (cm *CacheManager) Get(ctx context.Context, key string) ([]byte, error) {
	// L2: In-memory cache (fastest)
	if data, err := cm.l2Cache.Get(key); err == nil {
		return data, nil
	}

	// L1: Redis cache
	if data, err := cm.l1Cache.Get(ctx, key).Result(); err == nil {
		// Store in L2 cache for future fast access
		cm.l2Cache.Set(key, []byte(data))
		return []byte(data), nil
	}

	return nil, fmt.Errorf("key not found: %s", key)
}

// GetObject retrieves and deserializes an object from cache
func (cm *CacheManager) GetObject(ctx context.Context, key string, dest interface{}) error {
	data, err := cm.Get(ctx, key)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, dest)
}

// Set stores a value in the cache
func (cm *CacheManager) Set(ctx context.Context, key string, value []byte) error {
	// Store in both L1 and L2 caches
	err1 := cm.l1Cache.Set(ctx, key, value, cm.config.L1TTL).Err()
	err2 := cm.l2Cache.Set(key, value)

	if err1 != nil {
		return fmt.Errorf("failed to set L1 cache: %w", err1)
	}
	if err2 != nil {
		return fmt.Errorf("failed to set L2 cache: %w", err2)
	}

	return nil
}

// SetObject serializes and stores an object in cache
func (cm *CacheManager) SetObject(ctx context.Context, key string, value interface{}) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal object: %w", err)
	}

	return cm.Set(ctx, key, data)
}

// Delete removes a key from both cache levels
func (cm *CacheManager) Delete(ctx context.Context, key string) error {
	err1 := cm.l1Cache.Del(ctx, key).Err()
	err2 := cm.l2Cache.Delete(key)

	if err1 != nil {
		return fmt.Errorf("failed to delete from L1 cache: %w", err1)
	}
	if err2 != nil {
		return fmt.Errorf("failed to delete from L2 cache: %w", err2)
	}

	return nil
}

// Clear clears all cache levels
func (cm *CacheManager) Clear(ctx context.Context) error {
	// Clear L1 cache
	err1 := cm.l1Cache.FlushAll(ctx).Err()
	
	// Clear L2 cache
	err2 := cm.l2Cache.Reset()

	if err1 != nil {
		return fmt.Errorf("failed to clear L1 cache: %w", err1)
	}
	if err2 != nil {
		return fmt.Errorf("failed to clear L2 cache: %w", err2)
	}

	return nil
}

// GetStats returns cache statistics
func (cm *CacheManager) GetStats(ctx context.Context) map[string]interface{} {
	l1Stats := cm.l1Cache.Info(ctx, "memory").Val()
	l2Stats := cm.l2Cache.Stats()

	return map[string]interface{}{
		"l1_stats": l1Stats,
		"l2_stats": map[string]interface{}{
			"hits":   l2Stats.Hits,
			"misses": l2Stats.Misses,
			"size":   l2Stats.Size,
		},
	}
} 