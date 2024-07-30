dataloader-go
===

This is a implementation of a dataloader in Go.

- 200+ lines of code, easy to understand and maintain.
- 100% test coverage, bug free and reliable.
- Based on generics and can be used with any type of data.
- Use a lru cache to store the loaded values.
- Can be used to batch and cache multiple requests.

Installation
---

```go
import "github.com/sysulq/dataloader-go"
```

API Design
---

```go
// New creates a new DataLoader with the given loader and options.
func New[K comparable, V any](loader Loader[K, V], options ...Option) *DataLoader[K, V]

// Go loads a value for the given key. The value is returned in a channel.
func (d *DataLoader[K, V]) Go(ctx context.Context, key K) <-chan Result[V]
// Load loads a value for the given key. The value is returned in a Result.
func (d *DataLoader[K, V]) Load(ctx context.Context, key K) Result[V]
// LoadMany loads values for the given keys. The values are returned in a slice of Results.
func (d *DataLoader[K, V]) LoadMany(ctx context.Context, keys []K) []Result[V]
// LoadMap loads values for the given keys. The values are returned in a map of Results.
func (d *DataLoader[K, V]) LoadMap(ctx context.Context, keys []K) map[K]Result[V]
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
