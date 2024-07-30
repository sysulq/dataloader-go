package dataloader

import (
	"context"
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

// DataLoader is the main struct for the dataloader
type DataLoader[K comparable, V any] struct {
	loader Loader[K, V]
	cache  *expirable.LRU[K, V]
	config config
	mu     sync.Mutex
	batch  []K
	chs    []chan Result[V]
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
func New[K comparable, V any](loader Loader[K, V], options ...Option) *DataLoader[K, V] {
	config := config{
		BatchSize:   100,
		Wait:        16 * time.Millisecond,
		CacheSize:   1024,
		CacheExpire: time.Minute,
	}

	for _, option := range options {
		option(&config)
	}

	cache := expirable.NewLRU[K, V](config.CacheSize, nil, config.CacheExpire)

	return &DataLoader[K, V]{
		loader: loader,
		cache:  cache,
		config: config,
		batch:  make([]K, 0, config.BatchSize),
		chs:    make([]chan Result[V], 0, config.BatchSize),
	}
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

// AsyncLoad loads a single key asynchronously
func (d *DataLoader[K, V]) AsyncLoad(ctx context.Context, key K) <-chan Result[V] {
	ch := make(chan Result[V], 1)

	// Check if the key is in the cache
	if v, ok := d.cache.Get(key); ok {
		ch <- Result[V]{data: v}
		close(ch)
		return ch
	}

	// Lock the DataLoader
	d.mu.Lock()
	if len(d.batch) == 0 {
		// If there are no keys in the current batch, schedule a new batch timer
		go d.scheduleBatch(ctx, ch)
	}

	// If the current batch is full, start processing it
	if len(d.batch) >= d.config.BatchSize {
		// spawn a new goroutine to process the batch
		go d.processBatch(ctx, d.batch, d.chs)
		// Create a new batch, and a new set of channels
		d.batch = make([]K, 0, d.config.BatchSize)
		d.chs = make([]chan Result[V], 0, d.config.BatchSize)
	}

	// Add the key and channel to the current batch
	d.batch = append(d.batch, key)
	d.chs = append(d.chs, ch)

	// Unlock the DataLoader
	d.mu.Unlock()

	return ch
}

// Load loads a single key
func (d *DataLoader[K, V]) Load(ctx context.Context, key K) Result[V] {
	return <-d.AsyncLoad(ctx, key)
}

// LoadMany loads multiple keys
func (d *DataLoader[K, V]) LoadMany(ctx context.Context, keys []K) []Result[V] {
	chs := make([]<-chan Result[V], len(keys))
	for i, key := range keys {
		chs[i] = d.AsyncLoad(ctx, key)
	}

	results := make([]Result[V], len(keys))
	for i, ch := range chs {
		results[i] = <-ch
	}

	return results
}

// LoadMap loads multiple keys and returns a map of results
func (d *DataLoader[K, V]) LoadMap(ctx context.Context, keys []K) map[K]Result[V] {
	chs := make([]<-chan Result[V], len(keys))
	for i, key := range keys {
		chs[i] = d.AsyncLoad(ctx, key)
	}

	results := make(map[K]Result[V], len(keys))
	for i, ch := range chs {
		results[keys[i]] = <-ch
	}

	return results
}

// scheduleBatch schedules a batch to be processed
func (d *DataLoader[K, V]) scheduleBatch(ctx context.Context, ch chan Result[V]) {
	select {
	case <-time.After(d.config.Wait):
		d.mu.Lock()
		if len(d.batch) > 0 {
			go d.processBatch(ctx, d.batch, d.chs)
			d.batch = make([]K, 0, d.config.BatchSize)
			d.chs = make([]chan Result[V], 0, d.config.BatchSize)
		}
		d.mu.Unlock()
	case <-ctx.Done():
		ch <- Result[V]{err: ctx.Err()}
	}
}

// processBatch processes a batch of keys
func (d *DataLoader[K, V]) processBatch(ctx context.Context, keys []K, chs []chan Result[V]) {
	results := d.loader(ctx, keys)

	for i, key := range keys {
		if results[i].err == nil {
			d.cache.Add(key, results[i].data)
		}
		chs[i] <- results[i]
		close(chs[i])
	}
}

// Clear removes an item from the cache
func (d *DataLoader[K, V]) Clear(key K) {
	d.cache.Remove(key)
}

// ClearAll clears the entire cache
func (d *DataLoader[K, V]) ClearAll() {
	d.cache.Purge()
}
