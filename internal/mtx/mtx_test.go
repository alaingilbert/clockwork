package mtx

import (
	"errors"
	"testing"
)

func TestNewRWMtxAndGetSet(t *testing.T) {
	mt := NewRWMtx(42)

	if got := mt.Get(); got != 42 {
		t.Errorf("expected 42, got %v", got)
	}

	mt.Set(100)
	if got := mt.Get(); got != 100 {
		t.Errorf("expected 100 after Set, got %v", got)
	}
}

func TestGetPointer(t *testing.T) {
	mt := NewRWMtx("hello")

	ptr := mt.GetPointer()
	if *ptr != "hello" {
		t.Errorf("expected pointer to have value 'hello', got %v", *ptr)
	}

	// Modify directly via pointer
	*ptr = "world"
	if got := mt.Get(); got != "world" {
		t.Errorf("expected value to be 'world' after pointer modification, got %v", got)
	}
}

func TestRWith(t *testing.T) {
	mt := NewRWMtx(5)
	called := false

	mt.RWith(func(v int) {
		if v != 5 {
			t.Errorf("expected 5, got %v", v)
		}
		called = true
	})

	if !called {
		t.Error("callback in RWith was not called")
	}
}

func TestRWithE(t *testing.T) {
	mt := NewRWMtx(7)

	err := mt.RWithE(func(v int) error {
		if v != 7 {
			t.Errorf("expected 7, got %v", v)
		}
		return nil
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	expectedErr := errors.New("read error")
	err = mt.RWithE(func(v int) error {
		return expectedErr
	})
	if err != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
}

func TestWith(t *testing.T) {
	mt := NewRWMtx(10)

	mt.With(func(v *int) {
		*v += 5
	})

	if got := mt.Get(); got != 15 {
		t.Errorf("expected 15 after With, got %v", got)
	}
}

func TestWithE(t *testing.T) {
	mt := NewRWMtx(20)

	err := mt.WithE(func(v *int) error {
		*v += 3
		return nil
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if got := mt.Get(); got != 23 {
		t.Errorf("expected 23 after WithE, got %v", got)
	}

	expectedErr := errors.New("write error")
	err = mt.WithE(func(v *int) error {
		return expectedErr
	})
	if err != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
}
