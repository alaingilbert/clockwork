// Package mtx provides generic thread-safe wrappers for values using sync.RWMutex.
// It allows safe concurrent access and mutation of any value through helper methods.
package mtx

import "sync"

// RWMtx is a generic thread-safe wrapper for a value of type T using a RWMutex.
type RWMtx[T any] struct {
	sync.RWMutex
	v T
}

// NewRWMtx creates a new RWMtx instance with the given value.
func NewRWMtx[T any](v T) RWMtx[T] {
	return RWMtx[T]{v: v}
}

// Get returns a copy of the protected value.
// It acquires a read lock to ensure thread safety.
func (m *RWMtx[T]) Get() T {
	m.RLock()
	defer m.RUnlock()
	return m.v
}

// Set replaces the protected value with a new one.
// It acquires a write lock to ensure thread safety.
func (m *RWMtx[T]) Set(v T) {
	m.Lock()
	defer m.Unlock()
	m.v = v
}

// GetPointer returns a raw pointer to the protected value.
// It does NOT acquire any locks, so it is not safe to use in concurrent scenarios.
// Only use this when external synchronization is guaranteed.
func (m *RWMtx[T]) GetPointer() *T {
	return &m.v
}

// RWith executes a read-only callback with a copy of the protected value.
// It acquires a read lock to ensure thread safety.
func (m *RWMtx[T]) RWith(clb func(v T)) {
	_ = m.RWithE(func(tx T) error {
		clb(tx)
		return nil
	})
}

// RWithE executes a read-only callback with a copy of the protected value.
// It returns any error returned by the callback.
// It acquires a read lock to ensure thread safety.
func (m *RWMtx[T]) RWithE(clb func(v T) error) error {
	m.RLock()
	defer m.RUnlock()
	return clb(m.v)
}

// With executes a write callback with a pointer to the protected value.
// It acquires a write lock to ensure thread safety.
func (m *RWMtx[T]) With(clb func(v *T)) {
	_ = m.WithE(func(tx *T) error {
		clb(tx)
		return nil
	})
}

// WithE executes a write callback with a pointer to the protected value.
// It returns any error returned by the callback.
// It acquires a write lock to ensure thread safety.
func (m *RWMtx[T]) WithE(clb func(v *T) error) error {
	m.Lock()
	defer m.Unlock()
	return clb(&m.v)
}
