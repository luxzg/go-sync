package main

import (
	"context"
	"strconv"
	"sync"
	"time"
)

type memoryValue struct {
	value     string
	expiresAt time.Time
	hasExpiry bool
}

type MemoryRedis struct {
	mu sync.Mutex
	kv map[string]memoryValue
}

func NewMemoryRedis() *MemoryRedis {
	return &MemoryRedis{kv: map[string]memoryValue{}}
}

func (m *MemoryRedis) Set(_ context.Context, key string, val string, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry := memoryValue{value: val}
	if ttl > 0 {
		entry.hasExpiry = true
		entry.expiresAt = time.Now().Add(ttl)
	}
	m.kv[key] = entry
	return nil
}

func (m *MemoryRedis) Incr(_ context.Context, key string, subtract bool) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.dropIfExpiredLocked(key)

	current := 0
	if v, ok := m.kv[key]; ok {
		parsed, err := strconv.Atoi(v.value)
		if err != nil {
			return 0, err
		}
		current = parsed
	}

	if subtract {
		current--
	} else {
		current++
	}

	m.kv[key] = memoryValue{value: strconv.Itoa(current)}
	return current, nil
}

func (m *MemoryRedis) Get(_ context.Context, key string, deleteAfterGet bool) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.dropIfExpiredLocked(key)

	v, ok := m.kv[key]
	if !ok {
		return "", nil
	}
	if deleteAfterGet {
		delete(m.kv, key)
	}
	return v.value, nil
}

func (m *MemoryRedis) Del(_ context.Context, keys ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, key := range keys {
		delete(m.kv, key)
	}
	return nil
}

func (m *MemoryRedis) FlushAll(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.kv = map[string]memoryValue{}
	return nil
}

func (m *MemoryRedis) dropIfExpiredLocked(key string) {
	v, ok := m.kv[key]
	if !ok {
		return
	}
	if !v.hasExpiry {
		return
	}
	if time.Now().After(v.expiresAt) {
		delete(m.kv, key)
	}
}
