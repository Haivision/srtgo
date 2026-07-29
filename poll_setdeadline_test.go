package srtgo

import (
	"testing"
	"time"
)

// setDeadline() holds pd.lock and, for a deadline that has already passed,
// calls unblock(). While unblock() also took pd.lock this self-deadlocked on a
// non-reentrant sync.Mutex, wedging not just this socket -- Close() blocks on
// the same mutex -- but the process-wide pollServer.run goroutine with it.
//
// SetReadDeadline(time.Now()) is the canonical net.Conn idiom for aborting an
// in-flight read, so this was reachable through ordinary use.
func TestSetReadDeadlineInThePastDoesNotDeadlock(t *testing.T) {
	s := connectedPair(t)

	done := make(chan struct{})
	go func() {
		s.SetReadDeadline(time.Now())
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("SetReadDeadline(time.Now()) did not return within 5s: pd.lock self-deadlock")
	}
}

// The same, for the write side and for an explicitly past instant.
func TestSetWriteDeadlineInThePastDoesNotDeadlock(t *testing.T) {
	s := connectedPair(t)

	done := make(chan struct{})
	go func() {
		s.SetWriteDeadline(time.Now().Add(-1 * time.Second))
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("SetWriteDeadline(past) did not return within 5s: pd.lock self-deadlock")
	}
}

// A deadline set in the past must not merely return -- it must wake a reader
// that is already blocked, which is the entire point of the idiom.
func TestPastDeadlineWakesBlockedRead(t *testing.T) {
	s := connectedPair(t)

	type result struct {
		err error
		d   time.Duration
	}
	ch := make(chan result, 1)
	go func() {
		buf := make([]byte, 1316)
		start := time.Now()
		_, err := s.Read(buf)
		ch <- result{err, time.Since(start)}
	}()

	// Let the reader block in wait() before the deadline lands.
	time.Sleep(200 * time.Millisecond)
	s.SetReadDeadline(time.Now())

	select {
	case r := <-ch:
		if r.err == nil {
			t.Fatal("expected a timeout error, got nil")
		}
		if _, ok := r.err.(*SrtEpollTimeout); !ok {
			t.Fatalf("expected *SrtEpollTimeout, got %T: %v", r.err, r.err)
		}
		t.Logf("blocked Read woke after %v with err=%v", r.d.Round(time.Millisecond), r.err)
	case <-time.After(5 * time.Second):
		t.Fatal("a past deadline did not wake the blocked Read")
	}
}
