package tasks

import (
	"context"
	"fmt"
	lru "github.com/hashicorp/golang-lru"
)

// UserDataCacheMemory caches often needed User data
type UserDataCacheMemory struct {
	Cache *lru.Cache
}

// Add ads a UserDataCacheEntry to the cache
func (c *UserDataCacheMemory) Add(_ context.Context, key string, entry *UserDataCacheEntry) error {
	_ = c.Cache.Add(key, entry)
	return nil
}

// Invalidate removes a UserDataCacheEntry from the cache
func (c *UserDataCacheMemory) Invalidate(_ context.Context, key string) error {
	c.Cache.Remove(key)
	return nil
}

// Get retrieves a UserDataCacheEntry from the cache
func (c *UserDataCacheMemory) Get(_ context.Context, key string) (*UserDataCacheEntry, error) {
	result, ok := c.Cache.Get(key)
	if !ok {
		return nil, fmt.Errorf("could not find key %s in user cache", key)
	}

	userCache, ok := result.(*UserDataCacheEntry)
	if !ok {
		return nil, fmt.Errorf("cache entry was not a user cache entry")
	}

	return userCache, nil
}

// NewMockUserCache initializes a new UserDataCacheMemory
func NewMockUserCache() (*UserDataCacheMemory, error) {
	cache, err := lru.New(100)
	if err != nil {
		return nil, err
	}

	return &UserDataCacheMemory{
		Cache: cache,
	}, nil
}
