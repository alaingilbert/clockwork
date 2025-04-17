// Package clockwork contains a simple fake clock for Go.
package clockwork

import (
	"context"
	"errors"
	"github.com/jonboulle/clockwork/internal/mtx"
	"slices"
	"sort"
	"time"
)

// Clock provides an interface that packages can use instead of directly using
// the [time] module, so that chronology-related behavior can be tested.
type Clock interface {
	After(d time.Duration) <-chan time.Time
	Sleep(d time.Duration)
	Now() time.Time
	Since(t time.Time) time.Duration
	Until(t time.Time) time.Duration
	NewTicker(d time.Duration) Ticker
	NewTimer(d time.Duration) Timer
	AfterFunc(d time.Duration, f func()) Timer
}

// NewRealClock returns a Clock which simply delegates calls to the actual time
// package; it should be used by packages in production.
func NewRealClock() Clock {
	return &realClock{}
}

type realClock struct {
	loc *time.Location
}

// NewRealClockInLocation ...
func NewRealClockInLocation(location *time.Location) Clock {
	return &realClock{loc: location}
}

func (rc *realClock) After(d time.Duration) <-chan time.Time {
	return time.After(d)
}

func (rc *realClock) Sleep(d time.Duration) {
	time.Sleep(d)
}

func (rc *realClock) Now() (out time.Time) {
	out = time.Now()
	if rc.loc != nil {
		out = out.In(rc.loc)
	}
	return
}

func (rc *realClock) Since(t time.Time) time.Duration {
	return rc.Now().Sub(t)
}

func (rc *realClock) Until(t time.Time) time.Duration {
	return t.Sub(rc.Now())
}

func (rc *realClock) NewTicker(d time.Duration) Ticker {
	return realTicker{time.NewTicker(d)}
}

func (rc *realClock) NewTimer(d time.Duration) Timer {
	return realTimer{time.NewTimer(d)}
}

func (rc *realClock) AfterFunc(d time.Duration, f func()) Timer {
	return realTimer{time.AfterFunc(d, f)}
}

// FakeClock provides an interface for a clock which can be manually advanced
// through time.
//
// FakeClock maintains a list of "waiters," which consists of all callers
// waiting on the underlying clock (i.e. Tickers and Timers including callers of
// Sleep or After). Users can call BlockUntil to block until the clock has an
// expected number of waiters.
type FakeClock struct {
	inner mtx.RWMtx[fakeClockInner]
}

type fakeClockInner struct {
	waiters  []expirer
	blockers []*blocker
	time     time.Time
}

// NewFakeClock returns a FakeClock implementation which can be
// manually advanced through time for testing. The initial time of the
// FakeClock will be the current system time.
//
// Tests that require a deterministic time must use NewFakeClockAt.
func NewFakeClock() *FakeClock {
	return NewFakeClockAt(time.Now())
}

// NewFakeClockAt returns a FakeClock initialised at the given time.Time.
func NewFakeClockAt(t time.Time) *FakeClock {
	return &FakeClock{
		inner: mtx.NewRWMtx(fakeClockInner{
			time: t,
		}),
	}
}

// blocker is a caller of BlockUntil.
type blocker struct {
	count int
	// ch is closed when the underlying clock has the specified number of blockers.
	ch chan struct{}
}

// expirer is a timer or ticker that expires at some point in the future.
type expirer interface {
	// expire the expirer at the given time, returning the desired duration until
	// the next expiration, if any.
	expire(now time.Time) (next *time.Duration)

	// Get and set the expiration time.
	expiration() time.Time
	setExpiration(time.Time)
}

// After mimics [time.After]; it waits for the given duration to elapse on the
// fakeClock, then sends the current time on the returned channel.
func (fc *FakeClock) After(d time.Duration) <-chan time.Time {
	return fc.AfterNotify(d, make(chan struct{}))
}

// AfterNotify notifies when the waiters has been updated.
func (fc *FakeClock) AfterNotify(d time.Duration, ch chan struct{}) <-chan time.Time {
	c := fc.NewTimer(d).Chan()
	close(ch)
	return c
}

// Sleep blocks until the given duration has passed on the fakeClock.
func (fc *FakeClock) Sleep(d time.Duration) {
	fc.SleepNotify(d, make(chan struct{}))
}

// SleepNotify blocks until the given duration has passed on the fakeClock.
// Notify "ch" once the waiters has been updated
func (fc *FakeClock) SleepNotify(d time.Duration, ch chan struct{}) {
	afterCh := fc.After(d)
	close(ch)
	<-afterCh
}

// Now returns the current time of the fakeClock
func (fc *FakeClock) Now() (out time.Time) {
	fc.inner.RWith(func(v fakeClockInner) { out = v.time })
	return
}

// Since returns the duration that has passed since the given time on the
// fakeClock.
func (fc *FakeClock) Since(t time.Time) time.Duration {
	return fc.Now().Sub(t)
}

// Until returns the duration that has to pass from the given time on the fakeClock
// to reach the given time.
func (fc *FakeClock) Until(t time.Time) time.Duration {
	return t.Sub(fc.Now())
}

// NewTicker returns a Ticker that will expire only after calls to
// FakeClock.Advance() have moved the clock past the given duration.
//
// The duration d must be greater than zero; if not, NewTicker will panic.
func (fc *FakeClock) NewTicker(d time.Duration) Ticker {
	// Maintain parity with
	// https://cs.opensource.google/go/go/+/refs/tags/go1.20.3:src/time/tick.go;l=23-25
	if d <= 0 {
		panic(errors.New("non-positive interval for NewTicker"))
	}
	ft := newFakeTicker(fc, d)
	fc.inner.With(func(inner *fakeClockInner) {
		setExpirer(inner, ft, d)
	})
	return ft
}

// NewTimer returns a Timer that will fire only after calls to
// fakeClock.Advance() have moved the clock past the given duration.
func (fc *FakeClock) NewTimer(d time.Duration) Timer {
	t, _ := fc.newTimer(d, nil)
	return t
}

// AfterFunc mimics [time.AfterFunc]; it returns a Timer that will invoke the
// given function only after calls to fakeClock.Advance() have moved the clock
// past the given duration.
func (fc *FakeClock) AfterFunc(d time.Duration, f func()) Timer {
	t, _ := fc.newTimer(d, f)
	return t
}

// newTimer returns a new timer using an optional afterFunc and the time that
// timer expires.
func (fc *FakeClock) newTimer(d time.Duration, afterfunc func()) (*fakeTimer, time.Time) {
	ft := newFakeTimer(fc, afterfunc)
	fc.inner.With(func(inner *fakeClockInner) {
		setExpirer(inner, ft, d)
	})
	return ft, ft.expiration()
}

// newTimerAtTime is like newTimer, but uses a time instead of a duration.
//
// It is used to ensure FakeClock's lock is held constant through calling
// fc.After(t.Sub(fc.Now())). It should not be exposed externally.
func (fc *FakeClock) newTimerAtTime(t time.Time, afterfunc func()) *fakeTimer {
	ft := newFakeTimer(fc, afterfunc)
	fc.inner.With(func(inner *fakeClockInner) {
		setExpirer(inner, ft, t.Sub(inner.time))
	})
	return ft
}

// Advance advances fakeClock to a new point in time, ensuring waiters and
// blockers are notified appropriately before returning.
func (fc *FakeClock) Advance(d time.Duration) {
	fc.inner.With(func(inner *fakeClockInner) {
		end := inner.time.Add(d)
		// Expire the earliest waiter until the earliest waiter's expiration is after
		// end.
		//
		// We don't iterate because the callback of the waiter might register a new
		// waiter, so the list of waiters might change as we execute this.
		for len(inner.waiters) > 0 && !end.Before(inner.waiters[0].expiration()) {
			w := inner.waiters[0]
			inner.waiters = inner.waiters[1:]
			// Use the waiter's expiration as the current time for this expiration.
			now := w.expiration()
			inner.time = now
			if d := w.expire(now); d != nil {
				setExpirer(inner, w, *d) // Set the new expiration if needed.
			}
		}
		inner.time = end
	})
}

// BlockUntil blocks until the FakeClock has the given number of waiters.
//
// Prefer BlockUntilContext in new code, which offers context cancellation to
// prevent deadlock.
//
// Deprecated: New code should prefer BlockUntilContext.
func (fc *FakeClock) BlockUntil(n int) {
	_ = fc.BlockUntilContext(context.TODO(), n)
}

// BlockUntilContext blocks until the fakeClock has the given number of waiters
// or the context is cancelled.
func (fc *FakeClock) BlockUntilContext(ctx context.Context, n int) error {
	return fc.BlockUntilContextNotify(ctx, n, make(chan struct{}))
}

// BlockUntilContextNotify blocks until the fakeClock has the given number of waiters
// or the context is cancelled.
// Notify "ch" when blockers is updated.
func (fc *FakeClock) BlockUntilContextNotify(ctx context.Context, n int, ch chan struct{}) error {
	b := fc.newBlocker(n)
	close(ch)
	if b != nil {
		select {
		case <-b.ch:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

// Set up a new blocker to wait for more waiters.
// Only add if we don't already have n waiters.
func (fc *FakeClock) newBlocker(n int) (b *blocker) {
	fc.inner.With(func(inner *fakeClockInner) {
		if len(inner.waiters) < n {
			b = &blocker{count: n, ch: make(chan struct{})}
			inner.blockers = append(inner.blockers, b)
		}
	})
	return
}

// stop stops an expirer, returning true if the expirer was stopped.
func (fc *FakeClock) stop(e expirer) (stopped bool) {
	fc.inner.With(func(inner *fakeClockInner) {
		stopped = stopExpirer(inner, e)
	})
	return
}

// stopExpirer stops an expirer, returning true if the expirer was stopped.
func stopExpirer(inner *fakeClockInner, e expirer) (stopped bool) {
	if idx := slices.Index(inner.waiters, e); idx != -1 {
		inner.waiters = slices.Delete(inner.waiters, idx, idx+1)
		stopped = true
	}
	return
}

// setExpirer sets an expirer to expire at a future point in time.
func setExpirer(inner *fakeClockInner, e expirer, d time.Duration) {
	if d.Nanoseconds() <= 0 {
		// Special case for timers with duration <= 0: trigger immediately, never reset.
		// Tickers never get here, they panic if d is <= 0.
		e.expire(inner.time)
		return
	}
	// Add the expirer to the set of waiters and notify any blockers.
	e.setExpiration(inner.time.Add(d))
	idx := sort.Search(len(inner.waiters), func(i int) bool {
		return inner.waiters[i].expiration().After(e.expiration())
	})
	inner.waiters = slices.Insert(inner.waiters, idx, e)

	// Notify blockers of our new waiter.
	count := len(inner.waiters)
	inner.blockers = slices.DeleteFunc(inner.blockers, func(b *blocker) bool {
		if b.count <= count {
			close(b.ch)
			return true
		}
		return false
	})
}
