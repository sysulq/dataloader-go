package dataloader

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
