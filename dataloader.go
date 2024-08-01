package dataloader

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// Loader is the function type for loading data
type Loader[K comparable, V any] func(context.Context, []K) []Result[V]

// Interface is a `DataLoader` Interface which defines a public API for loading data from a particular
// data back-end with unique keys such as the `id` column of a SQL table or
// document name in a MongoDB database, given a batch loading function.
type Interface[K comparable, V any] interface {
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
	// TracerProvider is the tracer provider to use for tracing
	TracerProvider trace.TracerProvider
}

// dataLoader is the main struct for the dataloader
type dataLoader[K comparable, V any] struct {
	loader       Loader[K, V]
	cache        *expirable.LRU[K, V]
	config       config
	mu           sync.Mutex
	batch        []K
	batchCtx     []context.Context
	chs          map[K][]chan Result[V]
	stopSchedule chan struct{}
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
		loader:       loader,
		config:       config,
		stopSchedule: make(chan struct{}),
	}
	dl.reset()

	// Create a cache if the cache size is greater than 0
	if config.CacheSize > 0 {
		dl.cache = expirable.NewLRU[K, V](config.CacheSize, nil, config.CacheExpire)
	}

	return dl
}

// Load loads a single key
func (d *dataLoader[K, V]) Load(ctx context.Context, key K) Result[V] {
	ctx, span := d.startTrace(ctx, "dataLoader.Load")
	defer span.End()

	return <-d.goLoad(ctx, key)
}

// LoadMany loads multiple keys
func (d *dataLoader[K, V]) LoadMany(ctx context.Context, keys []K) []Result[V] {
	ctx, span := d.startTrace(ctx, "dataLoader.LoadMany")
	defer span.End()

	chs := make([]<-chan Result[V], len(keys))
	for i, key := range keys {
		chs[i] = d.goLoad(ctx, key)
	}

	results := make([]Result[V], len(keys))
	for i, ch := range chs {
		results[i] = <-ch
	}
	return results
}

// LoadMap loads multiple keys and returns a map of results
func (d *dataLoader[K, V]) LoadMap(ctx context.Context, keys []K) map[K]Result[V] {
	ctx, span := d.startTrace(ctx, "dataLoader.LoadMap")
	defer span.End()

	chs := make([]<-chan Result[V], len(keys))
	for i, key := range keys {
		chs[i] = d.goLoad(ctx, key)
	}

	results := make(map[K]Result[V], len(keys))
	for i, ch := range chs {
		results[keys[i]] = <-ch
	}
	return results
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

// goLoad loads a single key asynchronously
func (d *dataLoader[K, V]) goLoad(ctx context.Context, key K) <-chan Result[V] {
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
	if d.config.TracerProvider != nil {
		d.batchCtx = append(d.batchCtx, ctx)
	}

	if len(d.batch) == 0 {
		// If there are no keys in the current batch, schedule a new batch timer
		d.stopSchedule = make(chan struct{})
		go d.scheduleBatch(ctx, d.stopSchedule)
	} else {
		// Check if the key is in flight
		if chs, ok := d.chs[key]; ok {
			d.chs[key] = append(chs, ch)
			d.mu.Unlock()
			return ch
		}
	}

	// Add the key and channel to the current batch
	d.batch = append(d.batch, key)
	d.chs[key] = []chan Result[V]{ch}

	// If the current batch is full, start processing it
	if len(d.batch) >= d.config.BatchSize {
		// spawn a new goroutine to process the batch
		go d.processBatch(ctx, d.batch, d.batchCtx, d.chs)
		close(d.stopSchedule)
		// Create a new batch, and a new set of channels
		d.reset()
	}

	// Unlock the DataLoader
	d.mu.Unlock()
	return ch
}

// scheduleBatch schedules a batch to be processed
func (d *dataLoader[K, V]) scheduleBatch(ctx context.Context, stopSchedule <-chan struct{}) {
	select {
	case <-time.After(d.config.Wait):
		d.mu.Lock()
		if len(d.batch) > 0 {
			go d.processBatch(ctx, d.batch, d.batchCtx, d.chs)
			d.reset()
		}
		d.mu.Unlock()
	case <-stopSchedule:
		return
	}
}

// processBatch processes a batch of keys
func (d *dataLoader[K, V]) processBatch(ctx context.Context, keys []K, batchCtx []context.Context, chs map[K][]chan Result[V]) {
	defer func() {
		if r := recover(); r != nil {
			const size = 64 << 10
			buf := make([]byte, size)
			buf = buf[:runtime.Stack(buf, false)]
			err := fmt.Errorf("dataloader: panic received in loader function: %v", r)
			fmt.Fprintf(os.Stderr, "%v\n%s", err, buf)

			for _, chs := range chs {
				sendResult(chs, Result[V]{err: err})
			}
		}
	}()

	if d.config.TracerProvider != nil {
		// Create a span with links to the batch contexts, which enables trace propagation
		// We should deduplicate identical batch contexts to avoid creating duplicate links.
		links := make([]trace.Link, 0, len(keys))
		seen := make(map[context.Context]struct{}, len(batchCtx))
		for _, bCtx := range batchCtx {
			if _, ok := seen[bCtx]; ok {
				continue
			}
			links = append(links, trace.Link{SpanContext: trace.SpanContextFromContext(bCtx)})
			seen[bCtx] = struct{}{}
		}
		var span trace.Span
		ctx, span = d.startTrace(ctx, "dataLoader.Batch", trace.WithLinks(links...))
		defer span.End()
	}

	results := d.loader(ctx, keys)
	for i, key := range keys {
		if results[i].err == nil && d.cache != nil {
			d.cache.Add(key, results[i].data)
		}
		sendResult(chs[key], results[i])
	}
}

// reset resets the DataLoader
func (d *dataLoader[K, V]) reset() {
	d.batch = make([]K, 0, d.config.BatchSize)
	d.batchCtx = make([]context.Context, 0, d.config.BatchSize)
	d.chs = make(map[K][]chan Result[V], d.config.BatchSize)
}

// sendResult sends a result to channels
func sendResult[V any](chs []chan Result[V], result Result[V]) {
	for _, ch := range chs {
		ch <- result
		close(ch)
	}
}

// startTrace starts a trace span
func (d *dataLoader[K, V]) startTrace(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	if d.config.TracerProvider != nil {
		span := trace.SpanFromContext(ctx)
		if span.SpanContext().IsValid() {
			return d.config.TracerProvider.Tracer("dataLoader").Start(ctx, name, opts...)
		}
	}
	return ctx, noop.Span{}
}
