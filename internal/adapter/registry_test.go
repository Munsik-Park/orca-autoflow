package adapter_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/orca-cli/orca/internal/adapter"
)

// mockAdapter is a minimal stub that satisfies the Adapter interface for tests.
type mockAdapter struct{ name string }

func (m *mockAdapter) Name() string {
	return m.name
}

func (m *mockAdapter) Capabilities() adapter.Capabilities {
	return adapter.Capabilities{}
}

func (m *mockAdapter) Validate(_ context.Context) error {
	return nil
}

func (m *mockAdapter) Launch(_ context.Context, _ adapter.LaunchRequest) (adapter.Handle, error) {
	return adapter.Handle{}, nil
}

// newRegistry returns a fresh isolated registry for each test so tests don't share state.
func newRegistry() *adapter.Registry {
	return adapter.NewRegistry()
}

func TestRegistry_RoundTrip(t *testing.T) {
	t.Parallel()

	reg := newRegistry()
	factory := func(_ adapter.Config) (adapter.Adapter, error) {
		return &mockAdapter{name: "foo"}, nil
	}

	err := reg.Register("foo", factory)
	require.NoError(t, err)

	got, err := reg.Get("foo", adapter.Config{})
	require.NoError(t, err)
	assert.Equal(t, "foo", got.Name())
}

func TestRegistry_DuplicateRegister_ReturnsError(t *testing.T) {
	t.Parallel()

	reg := newRegistry()
	factory := func(_ adapter.Config) (adapter.Adapter, error) {
		return &mockAdapter{name: "bar"}, nil
	}

	require.NoError(t, reg.Register("bar", factory))

	err := reg.Register("bar", factory)
	require.Error(t, err)
	assert.True(t, errors.Is(err, adapter.ErrAlreadyRegistered), "expected ErrAlreadyRegistered, got %v", err)
}

func TestRegistry_UnknownAdapter_ReturnsError(t *testing.T) {
	t.Parallel()

	reg := newRegistry()
	_, err := reg.Get("unknown", adapter.Config{})
	require.Error(t, err)
	assert.True(t, errors.Is(err, adapter.ErrNotFound), "expected ErrNotFound, got %v", err)
}

func TestRegistry_FactoryError_Propagated(t *testing.T) {
	t.Parallel()

	reg := newRegistry()
	factoryErr := errors.New("adapter init failed")
	factory := func(_ adapter.Config) (adapter.Adapter, error) {
		return nil, factoryErr
	}

	require.NoError(t, reg.Register("broken", factory))

	_, err := reg.Get("broken", adapter.Config{})
	require.Error(t, err)
	assert.True(t, errors.Is(err, factoryErr))
}
