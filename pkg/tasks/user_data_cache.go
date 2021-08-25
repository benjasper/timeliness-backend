package tasks

import (
	"fmt"
	lru "github.com/hashicorp/golang-lru"
	"github.com/timeliness-app/timeliness-backend/pkg/tasks/calendar"
	"github.com/timeliness-app/timeliness-backend/pkg/users"
)

// UserDataCache caches often needed User data
type UserDataCache struct {
	Cache *lru.Cache
}

// UserDataCacheEntry holds calendar.RepositoryInterface
type UserDataCacheEntry struct {
	CalendarRepository calendar.RepositoryInterface
	User               *users.User
}

// NewUserCache initializes a new UserDataCache
func NewUserCache() (*UserDataCache, error) {
	cache, err := lru.New(10)
	if err != nil {
		return nil, err
	}

	return &UserDataCache{
		Cache: cache,
	}, nil
}

// Add adds a UserDataCacheEntry
func (c *UserDataCache) Add(key string, entry *UserDataCacheEntry) bool {
	return c.Cache.Add(key, entry)
}

// Contains checks if the cache contains an entry
func (c *UserDataCache) Contains(key string) bool {
	return c.Cache.Contains(key)
}

// Remove invalidates an entry
func (c *UserDataCache) Remove(key string) {
	c.Cache.Remove(key)
}

// RemoveOldest removes the oldest entry
func (c *UserDataCache) RemoveOldest() {
	c.Cache.RemoveOldest()
}

// Get retrieves a UserDataCacheEntry
func (c *UserDataCache) Get(key string) (*UserDataCacheEntry, error) {
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
