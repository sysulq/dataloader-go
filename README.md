dataloader-go
===

[![Go](https://github.com/sysulq/dataloader-go/actions/workflows/go.yml/badge.svg)](https://github.com/sysulq/dataloader-go/actions/workflows/go.yml)
[![codecov](https://codecov.io/gh/sysulq/dataloader-go/graph/badge.svg?token=KHQZ38ES45)](https://codecov.io/gh/sysulq/dataloader-go)

This is a Go implementation of Facebook's DataLoader.

A generic utility to be used as part of your application's data fetching layer to provide a consistent API over various backends and reduce the number of requests to the server.

Feature
---

- 200+ lines of code, easy to understand and maintain.
- 100% test coverage, bug free and reliable.
- Based on generics and can be used with any type of data.
- Use hashicorp/golang-lru to cache the loaded values.
- Can be used to batch and cache multiple requests.
- Deduplicate identical requests, reducing the number of requests.

Installation
---

```go
import "github.com/sysulq/dataloader-go"
```

API Design
---

```go
// New creates a new DataLoader with the given loader and options.
func New[K comparable, V any](loader Loader[K, V], options ...Option) Interface[K, V]

type Interface[K comparable, V any] interface {
	// Go loads a single key asynchronously
	Go(context.Context, K) <-chan Result[V]
	// Load loads a single key
	Load(context.Context, K) Result[V]
	// LoadMany loads multiple keys
	LoadMany(context.Context, []K) []Result[V]
	// LoadMap loads multiple keys and returns a map of results
	LoadMap(context.Context, []K) map[K]Result[V]
	// Clear removes an item from the cache
	Clear(K) Interface[K, V]
	// ClearAll clears the entire cache
	ClearAll() Interface[K, V]
	// Prime primes the cache with a key and value
	Prime(ctx context.Context, key K, value V) Interface[K, V]
}
```

Example
---

```go
package dataloader_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/sysulq/dataloader-go"
)

func TestExample(t *testing.T) {
	loader := dataloader.New(
		func(ctx context.Context, keys []int) []dataloader.Result[string] {
			results := make([]dataloader.Result[string], len(keys))

			for i, key := range keys {
				results[i] = dataloader.Wrap(fmt.Sprintf("Result for %d", key), nil)
			}
			return results
		},
		dataloader.WithCache(100, time.Minute),
		dataloader.WithBatchSize(50),
		dataloader.WithWait(5*time.Millisecond),
	)

	ctx := context.Background()

	// Load
	data, err := loader.Load(ctx, 1).Unwrap()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else {
		fmt.Printf("Result: %s\n", data)
		// Output:
		// Result: Result for 1
	}

	// LoadMany
	results := loader.LoadMany(ctx, []int{3, 4, 5})
	for _, result := range results {
		data, err := result.Unwrap()
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		} else {
			fmt.Printf("Result: %s\n", data)
			// Output:
			// Result: Result for 3
			// Result: Result for 4
			// Result: Result for 5
		}
	}

	// LoadMap
	keys := []int{6, 7, 8}
	resultsMap := loader.LoadMap(ctx, keys)
	for _, key := range keys {
		data, err := resultsMap[key].Unwrap()
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		} else {
			fmt.Printf("Result: %s\n", data)
			// Output:
			// Result: Result for 6
			// Result: Result for 7
			// Result: Result for 8
		}
	}

	// Prime
	loader.Prime(ctx, 8, "Prime result")
	data, err = loader.Load(ctx, 8).Unwrap()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else {
		fmt.Printf("Result: %s\n", data)
		// Output:
		// Result: Prime result
	}

	// Clear
	loader.Clear(7)
	data, err = loader.Load(ctx, 7).Unwrap()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else {
		fmt.Printf("Result: %s\n", data)
		// Output:
		// Result: Result for 7
	}

	// ClearAll
	loader.ClearAll()
	data, err = loader.Load(ctx, 8).Unwrap()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else {
		fmt.Printf("Result: %s\n", data)
		// Output:
		// Result: Result for 8
	}
}
```


Benchmark
---

```plain
goos: darwin
goarch: amd64
pkg: github.com/sysulq/dataloader-go
cpu: Intel(R) Core(TM) i5-10600K CPU @ 4.10GHz
BenchmarkDataLoader/direct.Batch-12         	 1437706	       827.1 ns/op	     480 B/op	      11 allocs/op
BenchmarkDataLoader/dataloader.Load-12      	  513562	      2386 ns/op	    1280 B/op	      20 allocs/op
BenchmarkDataLoader/dataloader.LoadMany-12  	  438864	      2500 ns/op	    1760 B/op	      23 allocs/op
BenchmarkDataLoader/dataloader.LoadMap-12   	  437780	      2711 ns/op	    2199 B/op	      24 allocs/op
PASS
coverage: 60.7% of statements
ok  	github.com/sysulq/dataloader-go	5.938s
```
