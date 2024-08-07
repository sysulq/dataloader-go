package redisloader

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/spf13/cast"
	"github.com/stretchr/testify/require"
	"github.com/sysulq/dataloader-go"
	"github.com/sysulq/dataloader-go/redisloader/mocks"
	"go.uber.org/mock/gomock"
)

// TestData represents a sample data structure for testing
type TestData struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Age  int    `json:"age"`
}

func TestLoadBatch(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	loader := func(ctx context.Context, keys []string) []dataloader.Result[TestData] {
		results := make([]dataloader.Result[TestData], len(keys))
		for i, key := range keys {
			results[i] = dataloader.Wrap(TestData{ID: i + 1, Name: "Loaded_" + key, Age: 30}, nil)
		}
		return results
	}
	defer client.FlushAll(context.Background())

	dl := New[string, TestData](client, loader)
	t.Run("All keys found in Redis", func(t *testing.T) {
		ctx := context.Background()
		keys := []string{"key1", "key2"}

		loadData := dl.Load(ctx, keys)
		require.Equal(t, 2, len(loadData))

		value1, _ := loadData[0].Unwrap()
		require.Equal(t, "Loaded_key1", value1.Name)
		value2, _ := loadData[1].Unwrap()
		require.Equal(t, "Loaded_key2", value2.Name)
	})

	t.Run("Empty keys slice", func(t *testing.T) {
		ctx := context.Background()
		keys := []string{}

		results := dl.Load(ctx, keys)

		require.Len(t, results, 0)
	})
}

func TestMocks(t *testing.T) {
	client := mocks.NewMockClientInterface(gomock.NewController(t))

	loader := func(ctx context.Context, keys []string) []dataloader.Result[TestData] {
		results := make([]dataloader.Result[TestData], len(keys))
		for i, key := range keys {
			results[i] = dataloader.Wrap(TestData{ID: i + 1, Name: "Loaded_" + key, Age: 30}, nil)
		}
		return results
	}

	dl := New[string, TestData](client, loader)

	t.Run("All keys found in Redis", func(t *testing.T) {
		ctx := context.Background()
		keys := []string{"key1", "key2"}

		jsonData1, _ := json.Marshal(TestData{ID: 1, Name: "John", Age: 30})
		jsonData2, _ := json.Marshal(TestData{ID: 2, Name: "Jane", Age: 25})

		sliceCmd := redis.NewSliceCmd(ctx)
		sliceCmd.SetVal([]interface{}{string(jsonData1), string(jsonData2)})
		client.EXPECT().MGet(ctx, keys).Return(sliceCmd)

		results := dl.Load(ctx, keys)

		require.Len(t, results, 2)
		value1, _ := results[0].Unwrap()
		require.Equal(t, TestData{ID: 1, Name: "John", Age: 30}, value1)
		value2, _ := results[1].Unwrap()
		require.Equal(t, TestData{ID: 2, Name: "Jane", Age: 25}, value2)
	})

	t.Run("Redis error", func(t *testing.T) {
		ctx := context.Background()
		keys := []string{"key1", "key2"}

		sliceCmd := redis.NewSliceCmd(ctx)
		sliceCmd.SetErr(redis.ErrClosed)
		client.EXPECT().MGet(ctx, keys).Return(sliceCmd)

		results := dl.Load(ctx, keys)

		require.Len(t, results, 2)
		value1, _ := results[0].Unwrap()
		require.Equal(t, TestData{ID: 1, Name: "Loaded_key1", Age: 30}, value1)
		value2, _ := results[1].Unwrap()
		require.Equal(t, TestData{ID: 2, Name: "Loaded_key2", Age: 30}, value2)
	})

	t.Run("Some keys missing in Redis", func(t *testing.T) {
		ctx := context.Background()
		keys := []string{"key1", "key2", "key3"}

		jsonData1, _ := json.Marshal(TestData{ID: 1, Name: "John", Age: 30})
		jsonData3, _ := json.Marshal(TestData{ID: 3, Name: "Bob", Age: 35})

		sliceCmd := redis.NewSliceCmd(ctx)
		sliceCmd.SetVal([]interface{}{string(jsonData1), nil, string(jsonData3)})
		client.EXPECT().MGet(ctx, keys).Return(sliceCmd)
		client.EXPECT().Pipelined(gomock.Any(), gomock.Any()).Return(nil, nil)

		results := dl.Load(ctx, keys)

		require.Len(t, results, 3)
		value1, _ := results[0].Unwrap()
		require.Equal(t, TestData{ID: 1, Name: "John", Age: 30}, value1)
		value2, _ := results[1].Unwrap()
		require.Equal(t, TestData{ID: 1, Name: "Loaded_key2", Age: 30}, value2)
		value3, _ := results[2].Unwrap()
		require.Equal(t, TestData{ID: 3, Name: "Bob", Age: 35}, value3)
	})

	t.Run("Pipeline error", func(t *testing.T) {
		ctx := context.Background()
		keys := []string{"key1", "key2"}

		sliceCmd := redis.NewSliceCmd(ctx)
		client.EXPECT().MGet(ctx, keys).Return(sliceCmd)
		client.EXPECT().Pipelined(gomock.Any(), gomock.Any()).Return(nil, redis.ErrClosed)

		results := dl.Load(ctx, keys)

		require.Len(t, results, 2)
		value1, _ := results[0].Unwrap()
		require.Equal(t, TestData{ID: 1, Name: "Loaded_key1", Age: 30}, value1)
		value2, _ := results[1].Unwrap()
		require.Equal(t, TestData{ID: 2, Name: "Loaded_key2", Age: 30}, value2)
	})
}

func TestNew(t *testing.T) {
	client := &mocks.MockClientInterface{}

	loader := func(ctx context.Context, keys []string) []dataloader.Result[TestData] {
		return nil
	}

	dl := New[string, TestData](client, loader,
		WithExpiration(time.Hour),
		WithKeyFunc(func(k any) string { return k.(string) }),
		WithMarshalFunc(json.Marshal),
		WithUnmarshalFunc(json.Unmarshal),
	)

	require.NotNil(t, dl)
	require.Equal(t, client, dl.redisClient)
	require.Equal(t, time.Hour, dl.opts.Expiration)
	require.NotNil(t, dl.opts.MarshalFunc)
	require.NotNil(t, dl.opts.UnmarshalFunc)
	require.NotNil(t, dl.opts.KeyFunc)
}

func TestPipeLineSet(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name      string
		keys      []string
		loadData  []dataloader.Result[string]
		setupMock func(*mocks.MockpipelinerInterface)
		expectErr bool
	}{
		{
			name: "Success case",
			keys: []string{"key1", "key2"},
			loadData: []dataloader.Result[string]{
				dataloader.Wrap("value1", nil),
				dataloader.Wrap("value2", nil),
			},
			setupMock: func(m *mocks.MockpipelinerInterface) {
				m.EXPECT().Set(ctx, "prefix:key1", gomock.Any(), time.Hour).Return(nil)
				m.EXPECT().Set(ctx, "prefix:key2", gomock.Any(), time.Hour).Return(nil)
			},
			expectErr: false,
		},
		{
			name: "Skip error results",
			keys: []string{"key1", "key2", "key3"},
			loadData: []dataloader.Result[string]{
				dataloader.Wrap("value1", nil),
				dataloader.Wrap("", errors.New("load error")),
				dataloader.Wrap("value3", nil),
			},
			setupMock: func(m *mocks.MockpipelinerInterface) {
				m.EXPECT().Set(ctx, "prefix:key1", gomock.Any(), time.Hour).Return(nil)
				m.EXPECT().Set(ctx, "prefix:key3", gomock.Any(), time.Hour).Return(nil)
			},
			expectErr: false,
		},
		{
			name: "Json marshal error",
			keys: []string{"key1"},
			loadData: []dataloader.Result[string]{
				dataloader.Wrap("fake error", nil),
			},
			setupMock: func(m *mocks.MockpipelinerInterface) {
			},
			expectErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockPipe := mocks.NewMockpipelinerInterface(gomock.NewController(t))
			tc.setupMock(mockPipe)

			dl := &redisDataLoader[string, string]{
				opts: option{
					KeyFunc: func(key any) string {
						return "prefix:" + cast.ToString(key)
					},
					MarshalFunc: func(a any) ([]byte, error) {
						if a == "fake error" {
							return nil, errors.New("fake error")
						}
						return json.Marshal(a)
					},
					Expiration: time.Hour,
				},
			}

			err := dl.pipeLineSet(ctx, mockPipe, tc.keys, tc.loadData)

			if tc.expectErr && err == nil {
				t.Error("Expected an error, but got none")
			} else if !tc.expectErr && err != nil {
				t.Errorf("Expected no error, but got: %v", err)
			}
		})
	}
}
