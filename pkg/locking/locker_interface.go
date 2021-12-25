package locking

import (
	"context"
	"time"
)

// LockerInterface represents a Locker
type LockerInterface interface {
	Acquire(ctx context.Context, key string, ttl time.Duration, tryOnlyOnce bool) (LockInterface, error)
}

// LockInterface represents a Lock
type LockInterface interface {
	Key() string
	Release(ctx context.Context) error
}
