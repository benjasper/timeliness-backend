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

// Add ads a UserDataCacheEntry to the cacheMemory
func (c *UserDataCacheMemory) Add(_ context.Context, key string, entry *UserDataCacheEntry) error {
	_ = c.Cache.Add(key, entry)
	return nil
}

// Invalidate removes a UserDataCacheEntry from the cacheMemory
func (c *UserDataCacheMemory) Invalidate(_ context.Context, key string) error {
	c.Cache.Remove(key)
	return nil
}

// Get retrieves a UserDataCacheEntry from the cacheMemory
func (c *UserDataCacheMemory) Get(_ context.Context, key string) (*UserDataCacheEntry, error) {
	result, ok := c.Cache.Get(key)
	if !ok {
		return nil, fmt.Errorf("could not find key %s in user cacheMemory", key)
	}

	userCache, ok := result.(*UserDataCacheEntry)
	if !ok {
		return nil, fmt.Errorf("cacheMemory entry was not a user cacheMemory entry")
	}

	return userCache, nil
}

// NewUserDataCacheMemory initializes a new UserDataCacheMemory
func NewUserDataCacheMemory(size int) (*UserDataCacheMemory, error) {
	cache, err := lru.New(size)
	if err != nil {
		return nil, err
	}

	return &UserDataCacheMemory{
		Cache: cache,
	}, nil
}
