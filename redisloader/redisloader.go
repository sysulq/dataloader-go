package redisloader

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/spf13/cast"
	"github.com/sysulq/dataloader-go"
)

//go:generate mockgen -source redisloader.go -destination mocks/mocks.go -package mocks
type ClientInterface interface {
	MGet(ctx context.Context, keys ...string) *redis.SliceCmd
	Pipelined(ctx context.Context, fn func(redis.Pipeliner) error) ([]redis.Cmder, error)
}

// redisDataLoader implements the DataLoader interface
type redisDataLoader[K comparable, V any] struct {
	redisClient ClientInterface
	loader      dataloader.Loader[K, V]
	opts        option
}

// option defines the options for the RedisDataLoader
type option struct {
	// Expiration is the expiration time for the redis cache
	// Default is 0, which means no expiration
	Expiration time.Duration
	// KeyFunc is the function to convert the key to a string
	KeyFunc func(any) string
	// MarshalFunc is the function to marshal the value
	// Default is json.Marshal, but can be changed to other marshal functions like proto.Marshal
	MarshalFunc func(any) ([]byte, error)
	// UnmarshalFunc is the function to unmarshal the value
	// Default is json.Unmarshal, but can be changed to other unmarshal functions like proto.Unmarshal
	UnmarshalFunc func([]byte, any) error
}

func WithExpiration(expiration time.Duration) func(*option) {
	return func(o *option) {
		o.Expiration = expiration
	}
}

func WithKeyFunc(keyFunc func(any) string) func(*option) {
	return func(o *option) {
		o.KeyFunc = keyFunc
	}
}

func WithMarshalFunc(marshalFunc func(any) ([]byte, error)) func(*option) {
	return func(o *option) {
		o.MarshalFunc = marshalFunc
	}
}

func WithUnmarshalFunc(unmarshalFunc func([]byte, any) error) func(*option) {
	return func(o *option) {
		o.UnmarshalFunc = unmarshalFunc
	}
}

// New creates a new RedisDataLoader
func New[K comparable, V any](client ClientInterface, loader dataloader.Loader[K, V], options ...func(*option)) *redisDataLoader[K, V] {
	opts := option{}
	for _, option := range options {
		option(&opts)
	}

	if opts.MarshalFunc == nil {
		opts.MarshalFunc = json.Marshal
	}
	if opts.UnmarshalFunc == nil {
		opts.UnmarshalFunc = json.Unmarshal
	}
	if opts.KeyFunc == nil {
		opts.KeyFunc = func(k any) string {
			return cast.ToString(k)
		}
	}

	return &redisDataLoader[K, V]{
		redisClient: client,
		loader:      loader,
		opts:        opts,
	}
}

// Load implements the DataLoader Loader function
func (dl *redisDataLoader[K, V]) Load(ctx context.Context, keys []K) []dataloader.Result[V] {
	resultMap := make(map[K]dataloader.Result[V], len(keys))
	if len(keys) == 0 {
		return mapToSlice(keys, resultMap)
	}

	missingKeys := make([]K, 0, len(keys))

	newKeys := make([]string, 0, len(keys))
	for _, key := range keys {
		newKeys = append(newKeys, dl.opts.KeyFunc(key))
	}

	// 1. try to get all keys from Redis
	redisValues, err := dl.redisClient.MGet(ctx, newKeys...).Result()
	if err != nil {
		// if there is an error, just load all keys from the loader
		loadData := dl.loader(ctx, keys)
		for i, key := range keys {
			resultMap[key] = loadData[i]
		}
		return mapToSlice(keys, resultMap)
	}

	// 2. process the values from Redis
	for i, key := range keys {
		if len(redisValues) <= i {
			missingKeys = append(missingKeys, key)
			continue
		}

		if redisValues[i] != nil {
			var value V
			err := dl.opts.UnmarshalFunc([]byte(redisValues[i].(string)), &value)
			resultMap[key] = dataloader.Wrap(value, err)
		} else {
			missingKeys = append(missingKeys, key)
		}
	}

	// 3. if there are missing keys, load them
	if len(missingKeys) > 0 && dl.loader != nil {

		// load the missing keys from the loader
		loadData := dl.loader(ctx, missingKeys)
		for idx, key := range missingKeys {
			resultMap[key] = loadData[idx]
		}

		// 4. save the missing keys to Redis
		_, err := dl.redisClient.Pipelined(ctx, func(pipe redis.Pipeliner) error {
			return dl.pipeLineSet(ctx, pipe, missingKeys, loadData)
		})
		if err != nil {
			return mapToSlice(keys, resultMap)
		}
	}

	return mapToSlice(keys, resultMap)
}

// pipelineInterface is an interface for the redis.Pipeliner
type pipelineInterface interface {
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd
}

// pipeLineSet sets the keys and loadData to Redis
func (dl *redisDataLoader[K, V]) pipeLineSet(ctx context.Context, pipe pipelineInterface, keys []K, loadData []dataloader.Result[V]) error {
	for idx, key := range keys {
		value, err := loadData[idx].Unwrap()
		if err != nil {
			continue
		}
		jsonValue, err := dl.opts.MarshalFunc(value)
		if err != nil {
			continue
		}
		pipe.Set(ctx, dl.opts.KeyFunc(key), jsonValue, dl.opts.Expiration)
	}

	return nil
}

// mapToSlice converts a map to a slice
func mapToSlice[K comparable, V any](keys []K, m map[K]V) []V {
	result := make([]V, len(keys))
	for i, key := range keys {
		result[i] = m[key]
	}
	return result
}
