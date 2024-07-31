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
- Use a LRU cache to store the loaded values.
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

Benchmark
---

```plain
goos: darwin
goarch: amd64
pkg: github.com/sysulq/dataloader-go
cpu: Intel(R) Core(TM) i5-10600K CPU @ 4.10GHz
BenchmarkDataLoader/direct.Batch-12         	 1233176	       824.4 ns/op	     480 B/op	      11 allocs/op
BenchmarkDataLoader/dataloader.Go-12 	          515784	      2258 ns/op	    1280 B/op	      20 allocs/op
BenchmarkDataLoader/dataloader.Load-12      	  501853	      2324 ns/op	    1280 B/op	      20 allocs/op
BenchmarkDataLoader/dataloader.LoadMany-12  	  457884	      2502 ns/op	    1680 B/op	      22 allocs/op
BenchmarkDataLoader/dataloader.LoadMap-12   	  445101	      2715 ns/op	    2119 B/op	      23 allocs/op
PASS
coverage: 73.8% of statements
ok  	github.com/sysulq/dataloader-go	6.384s
```
