package adapter

import (
	"errors"
	"fmt"
	"sync"
)

// ErrAlreadyRegistered is returned when Register is called with a name that
// already has a factory associated with it.
var ErrAlreadyRegistered = errors.New("adapter already registered")

// ErrNotFound is returned when Get is called with an unknown adapter name.
var ErrNotFound = errors.New("adapter not found")

// Factory is a constructor function that creates an Adapter from a config.
type Factory func(cfg Config) (Adapter, error)

// Registry maps adapter names to their Factory functions.
// Use NewRegistry to create an isolated instance.
type Registry struct {
	mu        sync.RWMutex
	factories map[string]Factory
}

// NewRegistry creates and returns an empty Registry.
func NewRegistry() *Registry {
	return &Registry{factories: make(map[string]Factory)}
}

// Register adds a factory for the given adapter name.
// It returns ErrAlreadyRegistered if the name is already taken.
func (r *Registry) Register(name string, f Factory) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.factories[name]; exists {
		return fmt.Errorf("%w: %s", ErrAlreadyRegistered, name)
	}
	r.factories[name] = f
	return nil
}

// Get instantiates the adapter registered under name using the provided config.
// It returns ErrNotFound if no factory is registered for name.
func (r *Registry) Get(name string, cfg Config) (Adapter, error) {
	r.mu.RLock()
	f, ok := r.factories[name]
	r.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, name)
	}
	return f(cfg)
}

// defaultRegistry is the process-level default registry, used by the package-level
// Register and Get functions for callers that don't need isolation.
var defaultRegistry = NewRegistry()

// Register adds a factory to the default registry.
func Register(name string, f Factory) error {
	return defaultRegistry.Register(name, f)
}

// Get instantiates an adapter from the default registry.
func Get(name string, cfg Config) (Adapter, error) {
	return defaultRegistry.Get(name, cfg)
}
