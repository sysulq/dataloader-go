package dataloader_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/sysulq/dataloader-go"
)

func exampleLoader(ctx context.Context, keys []int) []dataloader.Result[string] {
	results := make([]dataloader.Result[string], len(keys))

	for i, key := range keys {
		results[i] = dataloader.Wrap(fmt.Sprintf("Result for %d", key), nil)
	}
	return results
}

func TestExample(t *testing.T) {
	loader := dataloader.NewBatchedLoader(
		exampleLoader,
		dataloader.WithCache(100, time.Minute),
		dataloader.WithBatchSize(50),
		dataloader.WithWait(5*time.Millisecond),
	)

	ctx := context.Background()

	// Load
	data, err := loader.Load(ctx, 1).Unwrap()
	if err != nil {
		fmt.Printf("Error: %v\n", data)
	} else {
		fmt.Printf("Result: %s\n", data)
	}

	// AsyncLoad
	ch := loader.AsyncLoad(ctx, 2)
	result := <-ch

	fmt.Printf(result.Unwrap())

	// LoadMany
	results := loader.LoadMany(ctx, []int{3, 4, 5})
	for _, result := range results {
		data, err := result.Unwrap()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
		} else {
			fmt.Printf("Result: %s\n", data)
		}
	}

	// LoadMap
	resultsMap := loader.LoadMap(ctx, []int{6, 7, 8})
	for _, result := range resultsMap {
		data, err := result.Unwrap()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
		} else {
			fmt.Printf("Result: %s\n", data)
		}
	}
}
