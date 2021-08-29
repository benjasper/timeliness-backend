package locking

import (
	"context"
	"sync"
	"time"
)

// LockerMemory is a type of LockerInterface
type LockerMemory struct {
	pool  sync.Pool
	locks sync.Map
}

// NewLockerMemory builds a new LockerMemory instance
func NewLockerMemory() *LockerMemory {
	locker := LockerMemory{}
	locker.pool = sync.Pool{
		New: func() interface{} {
			return new(sync.RWMutex)
		},
	}

	return &locker
}

// Acquire acquires a LockInterface
func (l *LockerMemory) Acquire(_ context.Context, key string, _ time.Duration) (LockInterface, error) {
	l.getLock(key).Lock()

	return &LockMemory{
		key: key,
		release: func() {
			l.getLock(key).Unlock()
		},
	}, nil
}

func (l *LockerMemory) getLock(key interface{}) *sync.RWMutex {
	newLock := l.pool.Get()
	lock, stored := l.locks.LoadOrStore(key, newLock)
	if stored {
		l.pool.Put(newLock)
	}
	return lock.(*sync.RWMutex)
}

// LockMemory is a memory implementation of a LockInterface
type LockMemory struct {
	key     string
	release func()
}

// Key returns a key
func (l *LockMemory) Key() string {
	return l.key
}

// Release releases a LockMemory
func (l *LockMemory) Release(_ context.Context) error {
	l.release()
	return nil
}
