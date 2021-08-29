package tasks

import (
	"context"
	"github.com/go-redis/cache/v8"
	"github.com/go-redis/redis/v8"
	"github.com/timeliness-app/timeliness-backend/pkg/tasks/calendar"
	"github.com/timeliness-app/timeliness-backend/pkg/users"
	"time"
)

// UserDataCacheInterface is the interface for a UserDataCacheRedis
type UserDataCacheInterface interface {
	Add(ctx context.Context, key string, entry *UserDataCacheEntry) error
	Invalidate(ctx context.Context, key string) error
	Get(ctx context.Context, key string) (*UserDataCacheEntry, error)
}

// UserDataCacheRedis caches often needed User data
type UserDataCacheRedis struct {
	Cache *cache.Cache
}

// UserDataCacheEntry holds calendar.RepositoryInterface
type UserDataCacheEntry struct {
	CalendarRepository calendar.RepositoryInterface
	User               *users.User
}

// NewUserCacheRedis initializes a new UserDataCacheRedis
func NewUserCacheRedis(redisClient *redis.Client) (*UserDataCacheRedis, error) {
	redisCache := cache.New(&cache.Options{
		Redis: redisClient,
	})

	return &UserDataCacheRedis{
		Cache: redisCache,
	}, nil
}

// Add adds a UserDataCacheEntry
func (c *UserDataCacheRedis) Add(ctx context.Context, key string, entry *UserDataCacheEntry) error {
	err := c.Cache.Set(&cache.Item{
		Ctx:   ctx,
		Key:   key,
		Value: entry,
		TTL:   time.Minute * 10,
	})
	if err != nil {
		return err
	}

	return nil
}

// Invalidate invalidates an entry
func (c *UserDataCacheRedis) Invalidate(ctx context.Context, key string) error {
	err := c.Cache.Delete(ctx, key)
	if err != nil {
		return err
	}

	return nil
}

// Get retrieves a UserDataCacheEntry
func (c *UserDataCacheRedis) Get(ctx context.Context, key string) (*UserDataCacheEntry, error) {
	result := UserDataCacheEntry{}
	err := c.Cache.Get(ctx, key, &result)
	if err != nil {
		return nil, err
	}

	return &result, nil
}
