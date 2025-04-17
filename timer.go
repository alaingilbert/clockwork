package clockwork

import "time"

// Timer provides an interface which can be used instead of directly using
// [time.Timer]. The real-time timer t provides events through t.C which becomes
// t.Chan() to make this channel requirement definable in this interface.
type Timer interface {
	Chan() <-chan time.Time
	Reset(d time.Duration) bool
	Stop() bool
}

type realTimer struct{ *time.Timer }

func (r realTimer) Chan() <-chan time.Time { return r.C }

type fakeTimer struct {
	// The channel associated with the firer, used to send expiration times.
	c chan time.Time

	// The time when the firer expires. Only meaningful if the firer is currently
	// one of a FakeClock's waiters.
	exp time.Time

	// Fake clock
	fc *FakeClock

	// If present when the timer fires, the timer calls afterFunc in its own
	// goroutine rather than sending the time on Chan().
	afterFunc func()
}

func newFakeTimer(fc *FakeClock, afterfunc func()) *fakeTimer {
	return &fakeTimer{
		c:         make(chan time.Time, 1),
		fc:        fc,
		afterFunc: afterfunc,
	}
}

func (f *fakeTimer) Chan() <-chan time.Time { return f.c }

func (f *fakeTimer) Reset(d time.Duration) (stopped bool) {
	f.fc.inner.With(func(inner *fakeClockInner) {
		stopped = stopExpirer(inner, f)
		setExpirer(inner, f, d)
	})
	return
}

func (f *fakeTimer) Stop() bool { return f.fc.stop(f) }

func (f *fakeTimer) expire(now time.Time) *time.Duration {
	if f.afterFunc != nil {
		go f.afterFunc()
		return nil
	}

	// Never block on expiration.
	select {
	case f.c <- now:
	default:
	}
	return nil
}

func (f *fakeTimer) expiration() time.Time { return f.exp }

func (f *fakeTimer) setExpiration(t time.Time) { f.exp = t }
