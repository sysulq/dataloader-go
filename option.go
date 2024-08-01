package dataloader

import (
	"time"

	"go.opentelemetry.io/otel/trace"
)

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

// WithTracerProvider sets the tracer for the DataLoader
func WithTracerProvider(tp trace.TracerProvider) Option {
	return func(c *config) {
		c.TracerProvider = tp
	}
}
