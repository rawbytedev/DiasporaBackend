package cache_test

import (
	"Diaspora/internal/cache"
	"context"
	"fmt"
	"os"
	"runtime"
	"testing"
	"time"
)

type TestData struct {
	Name  string
	Value int
}

func TestCacheOperations(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cachetest-")
	if err != nil {
		t.Fatal(err)
	}
	cacheStore, err := cache.NewCache(tmpDir, nil)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	t.Cleanup(func() {
		if err := cacheStore.Close(); err != nil {
			t.Logf("Close error: %v", err)
		}
		runtime.GC()
		time.Sleep(100 * time.Millisecond)
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	tests := []struct {
		name      string
		operation func(*testing.T, *cache.CacheStore, context.Context) error
	}{
		{
			name: "Set and Get - successful retrieval",
			operation: func(t *testing.T, cs *cache.CacheStore, ctx context.Context) error {
				key := "test_key_1"
				data := TestData{Name: "Test", Value: 42}

				err := cs.Set(ctx, key, data)
				if err != nil {
					return fmt.Errorf("Set failed: %w", err)
				}

				var retrieved TestData
				err = cs.Get(ctx, key, &retrieved)
				if err != nil {
					return fmt.Errorf("Get failed: %w", err)
				}

				if retrieved.Name != data.Name || retrieved.Value != data.Value {
					return fmt.Errorf("data mismatch: expected %+v, got %+v", data, retrieved)
				}

				return nil
			},
		},
		{
			name: "Get - returns error for non-existent key",
			operation: func(t *testing.T, cs *cache.CacheStore, ctx context.Context) error {
				var data TestData
				err := cs.Get(ctx, "non_existent_key", &data)
				if err == nil {
					return fmt.Errorf("expected error for non-existent key, got nil")
				}

				return nil
			},
		},
		{
			name: "Delete - removes key",
			operation: func(t *testing.T, cs *cache.CacheStore, ctx context.Context) error {
				key := "delete_test_key"
				data := TestData{Name: "Delete", Value: 99}

				// Set value
				err := cs.Set(ctx, key, data)
				if err != nil {
					return fmt.Errorf("Set failed: %w", err)
				}

				// Delete value
				err = cs.Delete(ctx, key)
				if err != nil {
					return fmt.Errorf("Delete failed: %w", err)
				}

				// Try to get deleted value
				var retrieved TestData
				err = cs.Get(ctx, key, &retrieved)
				if err == nil {
					return fmt.Errorf("expected error after delete, got nil")
				}

				return nil
			},
		},
		{
			name: "InvalidatePrefix - removes all matching keys",
			operation: func(t *testing.T, cs *cache.CacheStore, ctx context.Context) error {
				prefix := "user:123:"
				keys := []string{prefix + "balance", prefix + "name", prefix + "email"}

				// Set values with prefix
				for _, key := range keys {
					err := cs.Set(ctx, key, "value")
					if err != nil {
						return fmt.Errorf("Set failed for %s: %w", key, err)
					}
				}

				// Invalidate prefix
				err := cs.InvalidatePrefix(ctx, prefix)
				if err != nil {
					return fmt.Errorf("InvalidatePrefix failed: %w", err)
				}

				// Verify all keys are deleted
				for _, key := range keys {
					var val string
					err = cs.Get(ctx, key, &val)
					if err == nil {
						return fmt.Errorf("expected error for key %s after invalidation, got nil", key)
					}
				}

				return nil
			},
		},
		{
			name: "Multiple types - stores and retrieves different types",
			operation: func(t *testing.T, cs *cache.CacheStore, ctx context.Context) error {
				// Store string
				err := cs.Set(ctx, "str_key", "hello world")
				if err != nil {
					return fmt.Errorf("Set string failed: %w", err)
				}

				// Store int
				err = cs.Set(ctx, "int_key", 42)
				if err != nil {
					return fmt.Errorf("Set int failed: %w", err)
				}

				// Store slice
				err = cs.Set(ctx, "slice_key", []string{"a", "b", "c"})
				if err != nil {
					return fmt.Errorf("Set slice failed: %w", err)
				}

				// Retrieve and verify
				var str string
				err = cs.Get(ctx, "str_key", &str)
				if err != nil || str != "hello world" {
					return fmt.Errorf("string retrieval failed: %w", err)
				}

				var num int
				err = cs.Get(ctx, "int_key", &num)
				if err != nil || num != 42 {
					return fmt.Errorf("int retrieval failed: %w", err)
				}

				var slice []string
				err = cs.Get(ctx, "slice_key", &slice)
				if err != nil || len(slice) != 3 {
					return fmt.Errorf("slice retrieval failed: %w", err)
				}

				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.operation(t, cacheStore, ctx); err != nil {
				t.Errorf("operation() error = %v", err)
			}
		})
	}
}

func TestConcurrency(t *testing.T) {
	cacheStore, err := cache.NewCache(t.TempDir(), nil)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer cacheStore.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	t.Run("Concurrent Set and Get - handles concurrent operations", func(t *testing.T) {
		done := make(chan bool)

		// Concurrent writes
		for i := 0; i < 10; i++ {
			go func(i int) {
				key := fmt.Sprintf("key_%d", i)
				_ = cacheStore.Set(ctx, key, i)
				done <- true
			}(i)
		}

		// Wait for all goroutines
		for i := 0; i < 10; i++ {
			<-done
		}

		// Verify all values
		for i := 0; i < 10; i++ {
			key := fmt.Sprintf("key_%d", i)
			var val int
			err := cacheStore.Get(ctx, key, &val)
			if err != nil || val != i {
				t.Errorf("concurrent test failed for key %s", key)
			}
		}
	})
}
