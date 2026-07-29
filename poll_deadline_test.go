package srtgo

import (
	"testing"
	"time"
)

// connectedPair brings up a non-blocking listener/caller pair on a random port
// and returns the accepted socket. The caller stays connected but never sends,
// so reads on the returned socket only complete via the deadline.
func connectedPair(t *testing.T) *SrtSocket {
	t.Helper()
	port := randomPort()
	ln := NewSrtSocket("127.0.0.1", port, map[string]string{"blocking": "0", "mode": "listener", "transtype": "file"})
	if ln == nil {
		t.Fatal("failed to create listener socket")
	}
	t.Cleanup(ln.Close)
	if err := ln.Listen(1); err != nil {
		t.Fatal("listen:", err)
	}

	stop := make(chan struct{})
	t.Cleanup(func() { close(stop) })
	go func() {
		c := NewSrtSocket("127.0.0.1", port, map[string]string{"blocking": "0", "mode": "caller", "transtype": "file"})
		if c == nil {
			return
		}
		defer c.Close()
		if err := c.Connect(); err != nil {
			return
		}
		<-stop
	}()

	remote, _, err := ln.Accept()
	if err != nil {
		t.Fatal("accept:", err)
	}
	t.Cleanup(remote.Close)
	return remote
}

// readAfter runs Read in a goroutine and reports how long it took. It fails the
// test rather than hanging if Read never returns.
func readAfter(t *testing.T, s *SrtSocket) (time.Duration, error) {
	t.Helper()
	type result struct {
		d   time.Duration
		err error
	}
	ch := make(chan result, 1)
	go func() {
		buf := make([]byte, 1316)
		start := time.Now()
		_, err := s.Read(buf)
		ch <- result{time.Since(start), err}
	}()
	select {
	case r := <-ch:
		return r.d, r.err
	case <-time.After(5 * time.Second):
		t.Fatal("Read did not return within 5s: the deadline error is being dropped and Read is spinning")
		return 0, nil
	}
}

// A read deadline must make Read block until the deadline and then report a
// timeout. Previously pollDesc timers were created with time.NewTimer(0), which
// fires immediately: the very first SetReadDeadline reset an already-expired,
// undrained timer, so wait() timed out at once. Read discarded that error, so
// it looped on EAsyncRCV forever.
func TestReadDeadlineExpires(t *testing.T) {
	s := connectedPair(t)

	s.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	d, err := readAfter(t, s)
	t.Logf("Read returned after %v with err=%v", d.Round(time.Millisecond), err)

	if err == nil {
		t.Fatal("expected a timeout error, got nil")
	}
	if _, ok := err.(*SrtEpollTimeout); !ok {
		t.Fatalf("expected *SrtEpollTimeout, got %T: %v", err, err)
	}
	if d < 250*time.Millisecond {
		t.Fatalf("Read returned after %v, want ~300ms: premature timeout", d)
	}
}

// A deadline that expires while no goroutine is inside Read leaves an undrained
// tick in the timer channel. setDeadline must stop *and* drain the timer, or the
// next deadline is observed as an immediate expiry.
func TestReadDeadlineAfterUnobservedExpiry(t *testing.T) {
	s := connectedPair(t)

	// Let a deadline lapse with no Read in flight.
	s.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	time.Sleep(300 * time.Millisecond)

	// A fresh deadline must be honoured in full.
	s.SetReadDeadline(time.Now().Add(1 * time.Second))
	d, err := readAfter(t, s)
	t.Logf("Read returned after %v with err=%v", d.Round(time.Millisecond), err)

	if d < 900*time.Millisecond {
		t.Fatalf("Read returned after %v, want ~1s: stale timer tick from the previous deadline", d)
	}
}

// Clearing a deadline with the zero time must restore blocking-until-ready
// behaviour, leaving no expiry behind from the deadline that was cleared.
func TestReadDeadlineCleared(t *testing.T) {
	s := connectedPair(t)

	s.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	if _, err := readAfter(t, s); err == nil {
		t.Fatal("expected a timeout error from the first read")
	}

	s.SetReadDeadline(time.Time{})
	type result struct {
		err error
	}
	ch := make(chan result, 1)
	go func() {
		buf := make([]byte, 1316)
		_, err := s.Read(buf)
		ch <- result{err}
	}()
	select {
	case r := <-ch:
		t.Fatalf("Read returned err=%v after the deadline was cleared, want it to keep waiting", r.err)
	case <-time.After(1 * time.Second):
		// Still waiting for data, as expected.
	}
}
