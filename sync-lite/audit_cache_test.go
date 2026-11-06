package main

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestAuditA07UnvisitedExpiredHints(t *testing.T) {
	m := NewMemoryRedis()
	ctx := context.Background()
	_ = m.Set(ctx, "expired", "1", time.Second)
	v := m.kv["expired"]
	v.expiresAt = time.Now().Add(-time.Hour)
	m.kv["expired"] = v
	_ = m.Set(ctx, "new", "2", time.Hour)
	if _, present := m.kv["expired"]; present {
		t.Fatal("unvisited expired hint retained")
	}
}

func TestAuditA07CapacityPreservesInterimCounts(t *testing.T) {
	m := NewMemoryRedis()
	ctx := context.Background()
	if _, err := m.Incr(ctx, "chain#interim_normal", false); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 1100; i++ {
		_ = m.Set(ctx, fmt.Sprintf("hint-%d", i), "1", time.Hour)
	}
	hints := 0
	for _, v := range m.kv {
		if v.hasExpiry {
			hints++
		}
	}
	if hints > 1024 {
		t.Fatalf("unbounded disposable hints: %d", hints)
	}
	value, err := m.Get(ctx, "chain#interim_normal", true)
	if err != nil || value != "1" {
		t.Fatal("pending interim count evicted", value, err)
	}
	value, err = m.Get(ctx, "chain#interim_normal", false)
	if err != nil || value != "" {
		t.Fatal("counter drain not atomic", value, err)
	}
}

func TestAuditA07ConcurrentHintsAndCounters(t *testing.T) {
	m := NewMemoryRedis()
	ctx := context.Background()
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				_ = m.Set(ctx, fmt.Sprintf("%d-%d", i, j), "1", time.Minute)
				_, _ = m.Incr(ctx, "interim", false)
			}
		}(i)
	}
	wg.Wait()
	got, err := m.Get(ctx, "interim", true)
	if err != nil || got != "320" {
		t.Fatal("lost concurrent interim counts", got, err)
	}
}
