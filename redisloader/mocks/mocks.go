// Code generated by MockGen. DO NOT EDIT.
// Source: redisloader.go
//
// Generated by this command:
//
//	mockgen -source redisloader.go -destination mocks/mocks.go -package mocks
//

// Package mocks is a generated GoMock package.
package mocks

import (
	context "context"
	reflect "reflect"
	time "time"

	redis "github.com/redis/go-redis/v9"
	gomock "go.uber.org/mock/gomock"
)

// MockClientInterface is a mock of ClientInterface interface.
type MockClientInterface struct {
	ctrl     *gomock.Controller
	recorder *MockClientInterfaceMockRecorder
}

// MockClientInterfaceMockRecorder is the mock recorder for MockClientInterface.
type MockClientInterfaceMockRecorder struct {
	mock *MockClientInterface
}

// NewMockClientInterface creates a new mock instance.
func NewMockClientInterface(ctrl *gomock.Controller) *MockClientInterface {
	mock := &MockClientInterface{ctrl: ctrl}
	mock.recorder = &MockClientInterfaceMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockClientInterface) EXPECT() *MockClientInterfaceMockRecorder {
	return m.recorder
}

// MGet mocks base method.
func (m *MockClientInterface) MGet(ctx context.Context, keys ...string) *redis.SliceCmd {
	m.ctrl.T.Helper()
	varargs := []any{ctx}
	for _, a := range keys {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "MGet", varargs...)
	ret0, _ := ret[0].(*redis.SliceCmd)
	return ret0
}

// MGet indicates an expected call of MGet.
func (mr *MockClientInterfaceMockRecorder) MGet(ctx any, keys ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx}, keys...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "MGet", reflect.TypeOf((*MockClientInterface)(nil).MGet), varargs...)
}

// Pipelined mocks base method.
func (m *MockClientInterface) Pipelined(ctx context.Context, fn func(redis.Pipeliner) error) ([]redis.Cmder, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Pipelined", ctx, fn)
	ret0, _ := ret[0].([]redis.Cmder)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Pipelined indicates an expected call of Pipelined.
func (mr *MockClientInterfaceMockRecorder) Pipelined(ctx, fn any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Pipelined", reflect.TypeOf((*MockClientInterface)(nil).Pipelined), ctx, fn)
}

// MockpipelinerInterface is a mock of pipelinerInterface interface.
type MockpipelinerInterface struct {
	ctrl     *gomock.Controller
	recorder *MockpipelinerInterfaceMockRecorder
}

// MockpipelinerInterfaceMockRecorder is the mock recorder for MockpipelinerInterface.
type MockpipelinerInterfaceMockRecorder struct {
	mock *MockpipelinerInterface
}

// NewMockpipelinerInterface creates a new mock instance.
func NewMockpipelinerInterface(ctrl *gomock.Controller) *MockpipelinerInterface {
	mock := &MockpipelinerInterface{ctrl: ctrl}
	mock.recorder = &MockpipelinerInterfaceMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockpipelinerInterface) EXPECT() *MockpipelinerInterfaceMockRecorder {
	return m.recorder
}

// Set mocks base method.
func (m *MockpipelinerInterface) Set(ctx context.Context, key string, value any, expiration time.Duration) *redis.StatusCmd {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Set", ctx, key, value, expiration)
	ret0, _ := ret[0].(*redis.StatusCmd)
	return ret0
}

// Set indicates an expected call of Set.
func (mr *MockpipelinerInterfaceMockRecorder) Set(ctx, key, value, expiration any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Set", reflect.TypeOf((*MockpipelinerInterface)(nil).Set), ctx, key, value, expiration)
}
