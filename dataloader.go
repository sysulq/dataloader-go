package dataloader

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
)

// Loader is the function type for loading data
type Loader[K comparable, V any] func(context.Context, []K) []Result[V]

// config holds the configuration for DataLoader
type config struct {
	// BatchSize is the number of keys to batch together, Default is 100
	BatchSize int
	// Wait is the duration to wait before processing a batch, Default is 16ms
	Wait time.Duration
	// CacheSize is the size of the cache, Default is 1024
	CacheSize int
	// CacheExpire is the duration to expire cache items, Default is 1 minute
	CacheExpire time.Duration
}

// dataLoader is the main struct for the dataloader
type dataLoader[K comparable, V any] struct {
	loader Loader[K, V]
	cache  *expirable.LRU[K, V]
	config config
	mu     sync.Mutex
	batch  []K
	chs    map[K][]chan Result[V]
}

// Interface is a `DataLoader` Interface which defines a public API for loading data from a particular
// data back-end with unique keys such as the `id` column of a SQL table or
// document name in a MongoDB database, given a batch loading function.
//
// Each `DataLoader` instance should contain a unique memoized cache. Use caution when
// used in long-lived applications or those which serve many users with
// different access permissions and consider creating a new instance per
// web request.
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

// Result is the result of a DataLoader operation
type Result[V any] struct {
	data V
	err  error
}

// Wrap wraps data and an error into a Result
func Wrap[V any](data V, err error) Result[V] {
	return Result[V]{data: data, err: err}
}

// TryUnwrap returns the data or an error
func (r Result[V]) Unwrap() (V, error) {
	return r.data, r.err
}

// New creates a new DataLoader with the given loader function and options
func New[K comparable, V any](loader Loader[K, V], options ...Option) Interface[K, V] {
	config := config{
		BatchSize:   100,
		Wait:        16 * time.Millisecond,
		CacheSize:   1024,
		CacheExpire: time.Minute,
	}

	for _, option := range options {
		option(&config)
	}

	dl := &dataLoader[K, V]{
		loader: loader,
		config: config,
	}

	dl.reset()

	// Create a cache if the cache size is greater than 0
	if config.CacheSize > 0 {
		dl.cache = expirable.NewLRU[K, V](config.CacheSize, nil, config.CacheExpire)
	}

	return dl
}

// Option is a function type for configuring DataLoader
type Option func(*config)

// WithCache sets the cache size for the DataLoader
func WithCache(size int, expire time.Duration) Option {
	return func(c *config) {
		c.CacheSize = size
		c.CacheExpire = expire
	}
}

// WithBatchSize sets the batch size for the DataLoader
func WithBatchSize(size int) Option {
	return func(c *config) {
		c.BatchSize = size
	}
}

// WithWait sets the wait duration for the DataLoader
func WithWait(wait time.Duration) Option {
	return func(c *config) {
		c.Wait = wait
	}
}

// Go loads a single key asynchronously
func (d *dataLoader[K, V]) Go(ctx context.Context, key K) <-chan Result[V] {
	ch := make(chan Result[V], 1)

	// Check if the key is in the cache
	if d.cache != nil {
		if v, ok := d.cache.Get(key); ok {
			ch <- Result[V]{data: v}
			close(ch)
			return ch
		}
	}

	// Lock the DataLoader
	d.mu.Lock()
	if len(d.batch) == 0 {
		// If there are no keys in the current batch, schedule a new batch timer
		go d.scheduleBatch(ctx, ch)
	}

	// Check if the key is in flight
	if chs, ok := d.chs[key]; ok {
		d.chs[key] = append(chs, ch)
		d.mu.Unlock()
		return ch
	}

	// If the current batch is full, start processing it
	if len(d.batch) >= d.config.BatchSize {
		// spawn a new goroutine to process the batch
		go d.processBatch(ctx, d.batch, d.chs)
		// Create a new batch, and a new set of channels
		d.reset()
	}

	// Add the key and channel to the current batch
	d.batch = append(d.batch, key)
	d.chs[key] = []chan Result[V]{ch}

	// Unlock the DataLoader
	d.mu.Unlock()

	return ch
}

// Load loads a single key
func (d *dataLoader[K, V]) Load(ctx context.Context, key K) Result[V] {
	return <-d.Go(ctx, key)
}

// LoadMany loads multiple keys
func (d *dataLoader[K, V]) LoadMany(ctx context.Context, keys []K) []Result[V] {
	chs := make([]<-chan Result[V], len(keys))
	for i, key := range keys {
		chs[i] = d.Go(ctx, key)
	}

	results := make([]Result[V], len(keys))
	for i, ch := range chs {
		results[i] = <-ch
	}

	return results
}

// LoadMap loads multiple keys and returns a map of results
func (d *dataLoader[K, V]) LoadMap(ctx context.Context, keys []K) map[K]Result[V] {
	chs := make([]<-chan Result[V], len(keys))
	for i, key := range keys {
		chs[i] = d.Go(ctx, key)
	}

	results := make(map[K]Result[V], len(keys))
	for i, ch := range chs {
		results[keys[i]] = <-ch
	}

	return results
}

// reset resets the DataLoader
func (d *dataLoader[K, V]) reset() {
	d.batch = make([]K, 0, d.config.BatchSize)
	d.chs = make(map[K][]chan Result[V], d.config.BatchSize)
}

// scheduleBatch schedules a batch to be processed
func (d *dataLoader[K, V]) scheduleBatch(ctx context.Context, ch chan Result[V]) {
	select {
	case <-time.After(d.config.Wait):
		d.mu.Lock()
		if len(d.batch) > 0 {
			go d.processBatch(ctx, d.batch, d.chs)
			d.reset()
		}
		d.mu.Unlock()
	case <-ctx.Done():
		ch <- Result[V]{err: ctx.Err()}
	}
}

// processBatch processes a batch of keys
func (d *dataLoader[K, V]) processBatch(ctx context.Context, keys []K, chs map[K][]chan Result[V]) {
	defer func() {
		if r := recover(); r != nil {
			const size = 64 << 10
			buf := make([]byte, size)
			buf = buf[:runtime.Stack(buf, false)]
			fmt.Fprintf(os.Stderr, "Dataloader: Panic received in loader function: %v\n%s", r, buf)

			for _, chs := range chs {
				for _, ch := range chs {
					ch <- Result[V]{err: fmt.Errorf("panic received in loader function: %v", r)}
					close(ch)
				}
			}
			return
		}
	}()
	results := d.loader(ctx, keys)

	for i, key := range keys {
		if results[i].err == nil && d.cache != nil {
			d.cache.Add(key, results[i].data)
		}

		for _, ch := range chs[key] {
			ch <- results[i]
			close(ch)
		}
	}
}

// Clear removes an item from the cache
func (d *dataLoader[K, V]) Clear(key K) Interface[K, V] {
	if d.cache != nil {
		d.cache.Remove(key)
	}

	return d
}

// ClearAll clears the entire cache
func (d *dataLoader[K, V]) ClearAll() Interface[K, V] {
	if d.cache != nil {
		d.cache.Purge()
	}

	return d
}

// Prime primes the cache with a key and value
func (d *dataLoader[K, V]) Prime(ctx context.Context, key K, value V) Interface[K, V] {
	if d.cache != nil {
		if _, ok := d.cache.Get(key); ok {
			d.cache.Add(key, value)
		}
	}

	return d
}
