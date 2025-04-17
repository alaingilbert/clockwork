package clockwork

import "time"

// Ticker provides an interface which can be used instead of directly using
// [time.Ticker]. The real-time ticker t provides ticks through t.C which
// becomes t.Chan() to make this channel requirement definable in this
// interface.
type Ticker interface {
	Chan() <-chan time.Time
	Reset(d time.Duration)
	Stop()
}

type realTicker struct{ *time.Ticker }

func (r realTicker) Chan() <-chan time.Time { return r.C }

type fakeTicker struct {
	// The channel associated with the firer, used to send expiration times.
	c chan time.Time

	// The time when the ticker expires. Only meaningful if the ticker is currently
	// one of a FakeClock's waiters.
	exp time.Time

	// Fake clock
	fc *FakeClock

	// The duration of the ticker.
	d time.Duration
}

func newFakeTicker(fc *FakeClock, d time.Duration) *fakeTicker {
	return &fakeTicker{
		c:  make(chan time.Time, 1),
		d:  d,
		fc: fc,
	}
}

func (f *fakeTicker) Chan() <-chan time.Time { return f.c }

func (f *fakeTicker) Reset(d time.Duration) {
	f.fc.inner.With(func(inner *fakeClockInner) {
		f.d = d
		setExpirer(inner, f, d)
	})
}

func (f *fakeTicker) Stop() { f.fc.stop(f) }

func (f *fakeTicker) expire(now time.Time) *time.Duration {
	// Never block on expiration.
	select {
	case f.c <- now:
	default:
	}
	return &f.d
}

func (f *fakeTicker) expiration() time.Time { return f.exp }

func (f *fakeTicker) setExpiration(t time.Time) { f.exp = t }
