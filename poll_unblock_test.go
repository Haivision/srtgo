package srtgo

import (
	"sync"
	"testing"
	"time"
)

func newTestPollDesc() *pollDesc {
	return &pollDesc{
		unblockRd: make(chan interface{}, 1),
		unblockWr: make(chan interface{}, 1),
		rdTimer:   time.NewTimer(noDeadline),
		wdTimer:   time.NewTimer(noDeadline),
	}
}

// unblock() must acquire no pollDesc lock, for every argument combination.
//
// setDeadline() calls unblock() while holding pd.lock (poll.go), so an unblock()
// that re-enters pd.lock self-deadlocks on a non-reentrant sync.Mutex. Today
// only the pollServer.run loop passes pollerr=true and it holds pollDescLock
// rather than pd.lock, so the pollerr branch is merely latent -- this test pins
// the invariant so that routing any pollerr=true call through a pd.lock-holding
// caller cannot silently reintroduce the deadlock PR #72 was written to fix.
func TestUnblockTakesNoPdLock(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mode    PollMode
		pollerr bool
		ioready bool
	}{
		{"read/pollerr", ModeRead, true, false},
		{"write/pollerr", ModeWrite, true, false},
		{"read/ioready", ModeRead, false, true},
		{"write/deadline", ModeWrite, false, false},
	} {
		tc := tc // the goroutine below outlives the iteration when it deadlocks
		t.Run(tc.name, func(t *testing.T) {
			pd := newTestPollDesc()
			done := make(chan struct{})
			go func() {
				defer close(done)
				// Exactly what setDeadline() does: hold pd.lock across unblock().
				pd.lock.Lock()
				defer pd.lock.Unlock()
				pd.unblock(tc.mode, tc.pollerr, tc.ioready)
			}()

			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatalf("unblock(pollerr=%v) did not return within 5s: it re-entered pd.lock, "+
					"which callers such as setDeadline already hold", tc.pollerr)
			}
		})
	}
}

// The pollErr flag must survive the lock-free path: unblock(pollerr=true) has to
// remain visible to checkPollErr(), otherwise dropping the lock would trade a
// deadlock for a lost error.
func TestUnblockPollErrIsVisibleToCheckPollErr(t *testing.T) {
	for _, mode := range []PollMode{ModeRead, ModeWrite} {
		pd := newTestPollDesc()
		if err := pd.checkPollErr(mode); err != nil {
			t.Fatalf("fresh pollDesc reported %v", err)
		}
		pd.unblock(mode, true, false)
		if _, ok := pd.checkPollErr(mode).(*SrtSocketClosed); !ok {
			t.Fatalf("mode %v: checkPollErr did not observe pollErr, got %v", mode, pd.checkPollErr(mode))
		}
	}
}

// pollErr is written by the pollServer goroutine from unblock() and read by
// checkPollErr() on user goroutines, with no lock in common. Under -race a
// half-converted field shows up here.
func TestUnblockPollErrConcurrentWithCheckPollErr(t *testing.T) {
	pd := newTestPollDesc()
	var wg sync.WaitGroup
	stop := make(chan struct{})

	wg.Add(3)
	go func() { // stands in for pollServer.run
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				pd.unblock(ModeRead, true, false)
				pd.unblock(ModeWrite, true, false)
			}
		}
	}()
	go func() { // stands in for a reader
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				pd.checkPollErr(ModeRead)
			}
		}
	}()
	go func() { // stands in for SetReadDeadline
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				pd.setDeadline(time.Now().Add(time.Hour), ModeRead)
			}
		}
	}()

	time.Sleep(200 * time.Millisecond)
	close(stop)
	wg.Wait()
}
