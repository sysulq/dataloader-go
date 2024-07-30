dataloader-go
===

This is a implementation of a dataloader in Go.

- 200 lines of code, easy to understand and maintain.
- 100% test coverage, bug free and reliable.
- Based on generics and can be used with any type of data.
- Use a lru cache to store the loaded values.
- Can be used to batch and cache multiple requests.

API Design
---

```go
// AsyncLoad loads a value for the given key. The value is returned in a channel.
func (d *DataLoader[K, V]) AsyncLoad(ctx context.Context, key K) <-chan Result[V]
// Load loads a value for the given key. The value is returned in a Result.
func (d *DataLoader[K, V]) Load(ctx context.Context, key K) Result[V]
// LoadMany loads values for the given keys. The values are returned in a slice of Results.
func (d *DataLoader[K, V]) LoadMany(ctx context.Context, keys []K) []Result[V]
// LoadMap loads values for the given keys. The values are returned in a map of Results.
func (d *DataLoader[K, V]) LoadMap(ctx context.Context, keys []K) map[K]Result[V]
```
