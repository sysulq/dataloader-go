package dataloader_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/sysulq/dataloader-go"
)

func TestExample(t *testing.T) {
	loader := dataloader.New(
		func(ctx context.Context, keys []int) []dataloader.Result[string] {
			results := make([]dataloader.Result[string], len(keys))

			for i, key := range keys {
				results[i] = dataloader.Wrap(fmt.Sprintf("Result for %d", key), nil)
			}
			return results
		},
		dataloader.WithCache(100, time.Minute),
		dataloader.WithBatchSize(50),
		dataloader.WithWait(5*time.Millisecond),
	)

	ctx := context.Background()

	// Load
	data, err := loader.Load(ctx, 1).Unwrap()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else {
		fmt.Printf("Result: %s\n", data)
		// Output:
		// Result: Result for 1
	}

	// Go
	ch := loader.Go(ctx, 2)
	result := <-ch
	data, err = result.Unwrap()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else {
		fmt.Printf("Result: %s\n", data)
		// Output:
		// Result: Result for 2
	}

	// LoadMany
	results := loader.LoadMany(ctx, []int{3, 4, 5})
	for _, result := range results {
		data, err := result.Unwrap()
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		} else {
			fmt.Printf("Result: %s\n", data)
			// Output:
			// Result: Result for 3
			// Result: Result for 4
			// Result: Result for 5
		}
	}

	// LoadMap
	keys := []int{6, 7, 8}
	resultsMap := loader.LoadMap(ctx, keys)
	for _, key := range keys {
		data, err := resultsMap[key].Unwrap()
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		} else {
			fmt.Printf("Result: %s\n", data)
			// Output:
			// Result: Result for 6
			// Result: Result for 7
			// Result: Result for 8
		}
	}

	// Prime
	loader.Prime(ctx, 8, "Prime result")
	data, err = loader.Load(ctx, 8).Unwrap()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else {
		fmt.Printf("Result: %s\n", data)
		// Output:
		// Result: Prime result
	}

	// Clear
	loader.Clear(7)
	data, err = loader.Load(ctx, 7).Unwrap()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else {
		fmt.Printf("Result: %s\n", data)
		// Output:
		// Result: Result for 7
	}

	// ClearAll
	loader.ClearAll()
	data, err = loader.Load(ctx, 8).Unwrap()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else {
		fmt.Printf("Result: %s\n", data)
		// Output:
		// Result: Result for 8
	}
}
