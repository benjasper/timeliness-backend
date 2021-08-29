package locking

import (
	"context"
	"github.com/bsm/redislock"
	"github.com/go-redis/redis/v8"
	"time"
)

// LockerRedis is a type of LockerInterface
type LockerRedis struct {
	locker *redislock.Client
}

// NewLockerRedis builds a new LockerRedis instance
func NewLockerRedis(redisClient *redis.Client) *LockerRedis {
	lockerRedis := LockerRedis{}
	lockerRedis.locker = redislock.New(redisClient)

	return &lockerRedis
}

// Acquire acquires a lock
func (l *LockerRedis) Acquire(ctx context.Context, key string, ttl time.Duration) (LockInterface, error) {
	backoff := redislock.LinearBackoff(500 * time.Millisecond)
	ctx, cancel := context.WithDeadline(ctx, time.Now().Add(time.Minute))
	defer cancel()

	obtain, err := l.locker.Obtain(ctx, key, ttl, &redislock.Options{
		RetryStrategy: backoff,
	})
	if err != nil {
		return nil, err
	}

	return &LockRedis{
		lock: obtain,
	}, nil
}

// LockRedis is a type of LockInterface
type LockRedis struct {
	lock *redislock.Lock
}

// Key Returns the key of the locking
func (l *LockRedis) Key() string {
	return l.lock.Key()
}

// Release will release the locking
func (l *LockRedis) Release(ctx context.Context) error {
	return l.lock.Release(ctx)
}
