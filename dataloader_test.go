package dataloader

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
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
	t.Run("Panic recovered", testPanicRecovered)
	t.Run("Prime", testPrime)
	t.Run("Inflight", testInflight)
	t.Run("Schedule batch", testScheduleBatch)
	t.Run("Trace", testTrace)
}

func testTrace(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))

	loader := New(func(ctx context.Context, keys []int) []Result[string] {
		results := make([]Result[string], len(keys))
		for i, key := range keys {
			results[i] = Result[string]{data: fmt.Sprintf("Result for %d", key)}
		}
		return results
	},
		WithTracerProvider(tp),
	)

	{
		ctx, span := tp.Tracer("dataLoader").Start(context.Background(), "test")
		defer span.End()

		wg := sync.WaitGroup{}
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				_ = loader.Load(ctx, i/2)
			}(i)
		}
		wg.Wait()

		spans := exporter.GetSpans()
		if len(spans.Snapshots()) != 11 {
			t.Errorf("Expected 11 spans, got %d", len(spans.Snapshots()))
		}

		haveBatch := false
		haveLoad := false
		loadCount := 0
		for _, s := range spans.Snapshots() {
			if s.Name() == "dataLoader.Batch" {
				haveBatch = true
				if len(s.Links()) != 10 {
					t.Errorf("Expected 10 links, got %d", len(s.Links()))
				}
			}
			if s.Name() == "dataLoader.Load" {
				loadCount++
				haveLoad = true
				if len(s.Links()) != 0 {
					t.Errorf("Expected 0 link, got %d", len(s.Links()))
				}
			}
		}

		if !haveBatch {
			t.Errorf("Expected to have a batch span")
		}

		if !haveLoad {
			t.Errorf("Expected to have a load span")
		}

		exporter.Reset()
	}
	{
		ctx, span := tp.Tracer("dataLoader").Start(context.Background(), "test")
		defer span.End()

		_ = loader.LoadMany(ctx, []int{1, 2, 3, 9})
		spans := exporter.GetSpans()

		if len(spans.Snapshots()) != 2 {
			t.Errorf("Expected 11 spans, got %d", len(spans.Snapshots()))
		}

		if spans.Snapshots()[0].Name() != "dataLoader.Batch" {
			t.Errorf("Unexpected span name: %v", spans.Snapshots()[0].Name())
		}

		if len(spans.Snapshots()[0].Links()) != 1 {
			t.Errorf("Expected 1 links, got %d", len(spans.Snapshots()[0].Links()))
		}

		if spans.Snapshots()[1].Name() != "dataLoader.LoadMany" {
			t.Errorf("Unexpected span name: %v", spans.Snapshots()[0].Name())
		}

		if len(spans.Snapshots()[1].Links()) != 0 {
			t.Errorf("Expected 0 links, got %d", len(spans.Snapshots()[0].Links()))
		}

		exporter.Reset()
	}
	{
		ctx, span := tp.Tracer("dataLoader").Start(context.Background(), "test")
		defer span.End()

		loader.LoadMap(ctx, []int{3, 14, 14, 15, 16})

		spans := exporter.GetSpans()

		if len(spans.Snapshots()) != 2 {
			t.Errorf("Expected 11 spans, got %d", len(spans.Snapshots()))
		}

		if spans.Snapshots()[0].Name() != "dataLoader.Batch" {
			t.Errorf("Unexpected span name: %v", spans.Snapshots()[0].Name())
		}

		if len(spans.Snapshots()[0].Links()) != 1 {
			t.Errorf("Expected 1 links, got %d", len(spans.Snapshots()[0].Links()))
		}

		if spans.Snapshots()[1].Name() != "dataLoader.LoadMap" {
			t.Errorf("Unexpected span name: %v", spans.Snapshots()[0].Name())
		}

		if len(spans.Snapshots()[1].Links()) != 0 {
			t.Errorf("Expected 0 links, got %d", len(spans.Snapshots()[0].Links()))
		}
	}
}

func testScheduleBatch(t *testing.T) {
	loader := New(func(ctx context.Context, keys []int) []Result[string] {
		if len(keys) != 5 {
			t.Errorf("Expected 5 keys, got %d", keys)
		}

		results := make([]Result[string], len(keys))
		for i, key := range keys {
			results[i] = Result[string]{data: fmt.Sprintf("Result for %d", key)}
		}
		return results
	}, WithBatchSize(5), WithWait(100*time.Millisecond))

	chs := make([]<-chan Result[string], 0)
	for i := 0; i < 4; i++ {
		chs = append(chs, loader.(*dataLoader[int, string]).goLoad(context.Background(), i))
	}
	time.Sleep(60 * time.Millisecond)

	for i := 4; i < 10; i++ {
		chs = append(chs, loader.(*dataLoader[int, string]).goLoad(context.Background(), i))
	}

	time.Sleep(60 * time.Millisecond)

	for idx, ch := range chs {
		result := <-ch
		data, err := result.Unwrap()
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		if data != fmt.Sprintf("Result for %d", idx) {
			t.Errorf("Unexpected result: %v", data)
		}
	}
}

func testInflight(t *testing.T) {
	loader := New(func(ctx context.Context, keys []int) []Result[string] {
		if len(keys) != 5 {
			t.Errorf("Expected 5 keys, got %d", keys)
		}

		results := make([]Result[string], len(keys))
		for i, key := range keys {
			results[i] = Result[string]{data: fmt.Sprintf("Result for %d", key)}
		}
		return results
	}, WithBatchSize(5))

	ii := make([]int, 0)
	for i := 0; i < 9; i++ {
		ii = append(ii, i/2)
	}

	chs := loader.LoadMany(context.TODO(), ii)
	for idx, result := range chs {
		data, err := result.Unwrap()
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		if data != fmt.Sprintf("Result for %d", idx/2) {
			t.Errorf("Unexpected result: %v", data)
		}
	}
}

func testBasicFunctionality(t *testing.T) {
	loader := New(func(ctx context.Context, keys []int) []Result[string] {
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
		t.Error(result.Unwrap())
	}
}

func testCaching(t *testing.T) {
	callCount := 0
	loader := New(func(ctx context.Context, keys []int) []Result[string] {
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
	loader := New(func(ctx context.Context, keys []int) []Result[string] {
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
	loader := New(func(ctx context.Context, keys []int) []Result[string] {
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
		t.Error(result.Unwrap())
	}
}

func testConcurrency(t *testing.T) {
	loader := New(func(ctx context.Context, keys []int) []Result[string] {
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
				t.Error(result.Unwrap())
			}
		}(i)
	}
	wg.Wait()
}

func testOptions(t *testing.T) {
	loader := New(func(ctx context.Context, keys []int) []Result[string] {
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
		t.Error(result.Unwrap())
	}

	// Other options are harder to test directly, but they've been set
}

func testContextCancellation(t *testing.T) {
	loader := New(func(ctx context.Context, keys []int) []Result[string] {
		results := make([]Result[string], len(keys))
		for i, key := range keys {
			results[i] = Result[string]{data: fmt.Sprintf("Result for %d", key)}
		}

		return results
	}, WithBatchSize(2))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	results := loader.LoadMany(ctx, []int{0, 1})
	for idx, result := range results {
		if result.err != nil {
			t.Errorf("Unexpected error: %v", result.err)
		}

		if result.data != fmt.Sprintf("Result for %d", idx) {
			t.Errorf("Unexpected result: %v", result.data)
		}
	}
}

func testClearAndClearAll(t *testing.T) {
	loader := New(func(ctx context.Context, keys []int) []Result[string] {
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
	loader := New(func(ctx context.Context, keys []int) []Result[string] {
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

	results = loader.LoadMany(context.Background(), []int{1, 2, 3, 4, 5, 6, 7, 8, 1})
	if results[0].err != nil || results[1].err != nil || results[2].err != nil {
		t.Errorf("Unexpected error: %v", results)
	}
	if len(results) != 9 {
		t.Errorf("Expected 3 results, got %d", len(results))
	}
	if results[0].data != "Result for 1" || results[1].data != "Result for 2" || results[2].data != "Result for 3" {
		t.Errorf("Unexpected results: %v", results)
	}

	if results[8].data != "Result for 1" {
		t.Errorf("Unexpected results: %v", results)
	}
}

func testLoadMap(t *testing.T) {
	loader := New(func(ctx context.Context, keys []int) []Result[string] {
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

	results = loader.LoadMap(context.Background(), []int{1, 2, 3, 4, 5, 6, 7, 8, 1})
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
		t.Errorf("Expected 8 results, got %d", len(results))
	}
}

func testPanicRecovered(t *testing.T) {
	loader := New(func(ctx context.Context, keys []int) []Result[string] {
		panic("Panic")
	})

	result, err := loader.Load(context.Background(), 1).Unwrap()
	if err == nil {
		t.Errorf("Expected error, got %v", result)
	}
}

func testPrime(t *testing.T) {
	loader := New(func(ctx context.Context, keys []int) []Result[string] {
		results := make([]Result[string], len(keys))
		for i, key := range keys {
			results[i] = Result[string]{data: fmt.Sprintf("Result for %d", key)}
		}
		return results
	})

	// Load
	data, err := loader.Load(context.Background(), 1).Unwrap()
	if err != nil {
		t.Errorf("Error: %v\n", data)
	}

	if data != "Result for 1" {
		t.Errorf("Unexpected result: %v\n", data)
	}

	// Prime
	data, err = loader.Prime(context.Background(), 1, "Prime for 1").Load(context.Background(), 1).Unwrap()
	if err != nil {
		t.Errorf("Error: %v\n", data)
	}

	if data != "Prime for 1" {
		t.Errorf("Unexpected result: %v\n", data)
	}
}
