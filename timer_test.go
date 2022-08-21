package clockwork

import (
	"testing"
	"time"
)

func TestFakeClockTimerStop(t *testing.T) {
	fc := &fakeClock{}

	ft := fc.NewTimer(1)
	ft.Stop()
	select {
	case <-ft.Chan():
		t.Errorf("received unexpected tick!")
	default:
	}
}

func TestFakeClockTimer_Race(t *testing.T) {
	fc := NewFakeClock()

	timer := fc.NewTimer(1 * time.Millisecond)
	defer timer.Stop()

	fc.Advance(1 * time.Millisecond)

	timeout := time.NewTimer(500 * time.Millisecond)
	defer timeout.Stop()

	select {
	case <-timer.Chan():
		// Pass
	case <-timeout.C:
		t.Fatalf("Timer didn't detect the clock advance!")
	}
}

func TestFakeClockTimer_Race2(t *testing.T) {
	fc := NewFakeClock()
	timer := fc.NewTimer(5 * time.Second)
	for i := 0; i < 100; i++ {
		fc.Advance(5 * time.Second)
		<-timer.Chan()
		timer.Reset(5 * time.Second)
	}
	timer.Stop()
}
