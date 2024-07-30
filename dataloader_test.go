package dataloader

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestDataLoader(t *testing.T) {
	t.Run("Basic functionality", testBasicFunctionality)
	t.Run("Caching", testCaching)
	t.Run("Batching", testBatching)
	t.Run("Error handling", testErrorHandling)
	t.Run("Concurrency", testConcurrency)
	t.Run("Options", testOptions)
	t.Run("Context cancellation", testContextCancellation)
	t.Run("Clear and ClearAll", testClearAndClearAll)
	t.Run("LoadMany", testLoadMany)
	t.Run("LoadMap", testLoadMap)
}

func testBasicFunctionality(t *testing.T) {
	loader := NewBatchedLoader(func(ctx context.Context, keys []int) []Result[string] {
		results := make([]Result[string], len(keys))
		for i, key := range keys {
			results[i] = Result[string]{data: fmt.Sprintf("Result for %d", key)}
		}
		return results
	})

	result := loader.Load(context.Background(), 1)
	if result.err != nil {
		t.Errorf("Unexpected error: %v", result.err)
	}

	if result.data != "Result for 1" {
		t.Errorf(result.Unwrap())
	}
}

func testCaching(t *testing.T) {
	callCount := 0
	loader := NewBatchedLoader(func(ctx context.Context, keys []int) []Result[string] {
		callCount++
		results := make([]Result[string], len(keys))
		for i, key := range keys {
			results[i] = Result[string]{data: fmt.Sprintf("Result for %d", key)}
		}
		return results
	})

	_ = loader.Load(context.Background(), 1)
	_ = loader.Load(context.Background(), 1)

	if callCount != 1 {
		t.Errorf("Expected 1 call, got %d", callCount)
	}
}

func testBatching(t *testing.T) {
	var mu sync.Mutex
	batchSizes := make([]int, 0)
	loader := NewBatchedLoader(func(ctx context.Context, keys []int) []Result[string] {
		mu.Lock()
		batchSizes = append(batchSizes, len(keys))
		mu.Unlock()
		results := make([]Result[string], len(keys))
		for i, key := range keys {
			results[i] = Result[string]{data: fmt.Sprintf("Result for %d", key)}
		}
		return results
	}, WithBatchSize(5), WithWait(10*time.Millisecond))

	var wg sync.WaitGroup
	for i := 0; i < 13; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_ = loader.Load(context.Background(), i)
		}(i)
	}
	wg.Wait()

	// Wait a bit to ensure all batches are processed
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	totalKeys := 0
	for _, size := range batchSizes {
		totalKeys += size
	}

	if totalKeys != 13 {
		t.Errorf("Expected to process 13 keys in total, but processed %d", totalKeys)
	}

	if len(batchSizes) < 2 {
		t.Errorf("Expected at least 2 batches, got %d", len(batchSizes))
	}

	for i, size := range batchSizes {
		if size > 5 {
			t.Errorf("Batch %d exceeded maximum size: %d", i, size)
		}
	}

	t.Logf("Batch sizes: %v", batchSizes)
}

func testErrorHandling(t *testing.T) {
	loader := NewBatchedLoader(func(ctx context.Context, keys []int) []Result[string] {
		results := make([]Result[string], len(keys))
		for i, key := range keys {
			if key%2 == 0 {
				results[i] = Result[string]{err: fmt.Errorf("Error for %d", key)}
			} else {
				results[i] = Result[string]{data: fmt.Sprintf("Result for %d", key)}
			}
		}
		return results
	})

	result := loader.Load(context.Background(), 2)
	if result.err == nil || result.err.Error() != "Error for 2" {
		t.Errorf("Expected error for even number, got: %v", result.err)
	}

	result = loader.Load(context.Background(), 1)
	if result.err != nil {
		t.Errorf("Unexpected error for odd number: %v", result.err)
	}
	if result.data != "Result for 1" {
		t.Errorf(result.Unwrap())
	}
}

func testConcurrency(t *testing.T) {
	loader := NewBatchedLoader(func(ctx context.Context, keys []int) []Result[string] {
		time.Sleep(100 * time.Millisecond) // Simulate some work
		results := make([]Result[string], len(keys))
		for i, key := range keys {
			results[i] = Result[string]{data: fmt.Sprintf("Result for %d", key)}
		}
		return results
	})

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			result := loader.Load(context.Background(), i)
			if result.err != nil {
				t.Errorf("Unexpected error: %v", result.err)
			}
			if result.data != fmt.Sprintf("Result for %d", i) {
				t.Errorf(result.Unwrap())
			}
		}(i)
	}
	wg.Wait()
}

func testOptions(t *testing.T) {
	loader := NewBatchedLoader(func(ctx context.Context, keys []int) []Result[string] {
		results := make([]Result[string], len(keys))
		for i, key := range keys {
			results[i] = Result[string]{data: fmt.Sprintf("Result for %d", key)}
		}
		return results
	}, WithCache(10, time.Minute), WithBatchSize(5), WithWait(50*time.Millisecond))

	// Test cache size
	for i := 0; i < 15; i++ {
		_ = loader.Load(context.Background(), i)
	}
	result := loader.Load(context.Background(), 0)
	if result.data != "Result for 0" {
		t.Errorf(result.Unwrap())
	}

	// Other options are harder to test directly, but they've been set
}

func testContextCancellation(t *testing.T) {
	loader := NewBatchedLoader(func(ctx context.Context, keys []int) []Result[string] {
		<-ctx.Done()
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result := loader.Load(ctx, 1)
	if result.err == nil {
		t.Error("Expected error when context is cancelled")
	}
}

func testClearAndClearAll(t *testing.T) {
	loader := NewBatchedLoader(func(ctx context.Context, keys []int) []Result[string] {
		results := make([]Result[string], len(keys))
		for i, key := range keys {
			results[i] = Result[string]{data: fmt.Sprintf("Result for %d", key)}
		}
		return results
	})

	_ = loader.Load(context.Background(), 1)
	_ = loader.Load(context.Background(), 2)

	loader.Clear(1)
	result := loader.Load(context.Background(), 1)
	if result.err != nil {
		t.Errorf("Unexpected error: %v", result.err)
	}

	loader.ClearAll()
	result = loader.Load(context.Background(), 2)
	if result.err != nil {
		t.Errorf("Unexpected error: %v", result.err)
	}
}

func testLoadMany(t *testing.T) {
	loader := NewBatchedLoader(func(ctx context.Context, keys []int) []Result[string] {
		results := make([]Result[string], len(keys))
		for i, key := range keys {
			results[i] = Result[string]{data: fmt.Sprintf("Result for %d", key)}
		}
		return results
	}, WithBatchSize(5))

	results := loader.LoadMany(context.Background(), []int{1, 2, 3})
	if results[0].err != nil || results[1].err != nil || results[2].err != nil {
		t.Errorf("Unexpected error: %v", results)
	}
	if len(results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(results))
	}
	if results[0].data != "Result for 1" || results[1].data != "Result for 2" || results[2].data != "Result for 3" {
		t.Errorf("Unexpected results: %v", results)
	}

	results = loader.LoadMany(context.Background(), []int{1, 2, 3, 4, 5, 6, 7, 8})
	if results[0].err != nil || results[1].err != nil || results[2].err != nil {
		t.Errorf("Unexpected error: %v", results)
	}
	if len(results) != 8 {
		t.Errorf("Expected 3 results, got %d", len(results))
	}
	if results[0].data != "Result for 1" || results[1].data != "Result for 2" || results[2].data != "Result for 3" {
		t.Errorf("Unexpected results: %v", results)
	}
}

func testLoadMap(t *testing.T) {
	loader := NewBatchedLoader(func(ctx context.Context, keys []int) []Result[string] {
		results := make([]Result[string], len(keys))
		for i, key := range keys {
			results[i] = Result[string]{data: fmt.Sprintf("Result for %d", key)}
		}
		return results
	})

	results := loader.LoadMap(context.Background(), []int{1, 2, 3})
	for k, v := range results {
		data, err := v.Unwrap()
		if err != nil {
			t.Errorf("Unexpected error: %v", v)
		}
		if data != fmt.Sprintf("Result for %d", k) {
			t.Errorf("Unexpected result: %v", v)
		}
	}

	if len(results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(results))
	}

	results = loader.LoadMap(context.Background(), []int{1, 2, 3, 4, 5, 6, 7, 8})
	for k, v := range results {
		data, err := v.Unwrap()
		if err != nil {
			t.Errorf("Unexpected error: %v", v)
		}
		if data != fmt.Sprintf("Result for %d", k) {
			t.Errorf("Unexpected result: %v", v)
		}
	}

	if len(results) != 8 {
		t.Errorf("Expected 3 results, got %d", len(results))
	}
}
