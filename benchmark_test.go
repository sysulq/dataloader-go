package dataloader

import (
	"context"
	"fmt"
	"testing"
)

func BenchmarkDataLoader(b *testing.B) {
	batchFunc := func(ctx context.Context, keys []int) []Result[string] {
		results := make([]Result[string], len(keys))
		for i, key := range keys {
			results[i] = Result[string]{data: fmt.Sprintf("Result for %d", key)}
		}
		return results
	}

	loader := New(batchFunc)

	b.Run("direct.Batch", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = batchFunc(context.Background(), []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10})
		}
	})

	b.Run("dataloader.Load", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for j := 0; j < 10; j++ {
				_ = loader.Load(context.Background(), j)
			}
		}
	})

	b.Run("dataloader.LoadMany", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = loader.LoadMany(context.Background(), []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10})
		}
	})

	b.Run("dataloader.LoadMap", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = loader.LoadMap(context.Background(), []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10})
		}
	})
}
