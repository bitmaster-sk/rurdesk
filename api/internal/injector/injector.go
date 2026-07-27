package injector

import (
	"sync"
)

type dependency struct {
	instance any
	err      error
	creating bool
	ready    chan struct{}
}

type DependencyInjector struct {
	lock sync.RWMutex
	deps map[string]*dependency
}

func NewDependencyInjector() *DependencyInjector {
	return &DependencyInjector{
		deps: make(map[string]*dependency),
	}
}

// GetWithNew returns the dependency for key, creating it via factory if absent.
//
// WARNING: use through typed accessor functions, not directly.
func (di *DependencyInjector) GetWithNew(key string, factory func() (any, error)) (any, error) {
	for {
		di.lock.RLock()
		state, exists := di.deps[key]
		di.lock.RUnlock()

		if !exists {
			// Try to become the creator.
			di.lock.Lock()

			// Re-check: another goroutine may have created it while we waited for the lock.
			if state, exists = di.deps[key]; exists {
				// We lost the race; fall through to wait for the actual creator.
				di.lock.Unlock()
			} else {
				// We are the creator.
				state = &dependency{
					creating: true,
					ready:    make(chan struct{}),
				}
				di.deps[key] = state
				di.lock.Unlock()

				// Run the factory without holding the lock.
				instance, err := factory()

				// Publish the result and wake any waiters.
				di.lock.Lock()
				state.instance = instance
				state.err = err
				state.creating = false
				close(state.ready)
				di.lock.Unlock()

				return instance, err
			}
		}

		if state.creating {
			// Another goroutine is creating it; wait for it to finish.
			<-state.ready
			return state.instance, state.err
		} else {
			return state.instance, state.err
		}
	}
}

// Set injects a dependency directly.
//
// WARNING: for tests only; do not use in production code.
func (di *DependencyInjector) Set(key string, instance any) {
	state := &dependency{
		instance: instance,
		creating: false,
		ready:    make(chan struct{}),
	}
	close(state.ready)

	di.lock.Lock()
	defer di.lock.Unlock()
	di.deps[key] = state
}

// Clear removes a single dependency.
//
// WARNING: for tests only; do not use in production code.
func (di *DependencyInjector) Clear(key string) {
	di.lock.Lock()
	defer di.lock.Unlock()
	delete(di.deps, key)
}

// Peek returns an already-constructed dependency, or false when the key was
// never built. Unlike GetWithNew it never runs a factory, so a caller shutting
// dependencies down cannot accidentally create the very thing it is closing.
func (di *DependencyInjector) Peek(key string) (any, bool) {
	di.lock.RLock()
	defer di.lock.RUnlock()
	state, exists := di.deps[key]
	if !exists || state.creating || state.err != nil {
		return nil, false
	}
	return state.instance, true
}

// ClearAll removes all dependencies.
//
// WARNING: for tests only; do not use in production code.
func (di *DependencyInjector) ClearAll() {
	di.lock.Lock()
	defer di.lock.Unlock()
	di.deps = make(map[string]*dependency)
}
