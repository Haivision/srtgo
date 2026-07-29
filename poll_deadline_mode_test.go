package srtgo

import (
	"testing"
	"time"
)

// deadlines reads the two deadline fields under pd.lock, the same lock
// setDeadline holds while writing them.
func deadlines(s *SrtSocket) (rd, wd int64) {
	s.pd.lock.Lock()
	defer s.pd.lock.Unlock()
	return s.pd.rdDeadline, s.pd.wdDeadline
}

// approx reports whether d is within tolerance of want.
func approx(d int64, want, tolerance time.Duration) bool {
	diff := time.Duration(d) - want
	if diff < 0 {
		diff = -diff
	}
	return diff <= tolerance
}

// Each of the three public deadline setters must touch exactly the deadlines
// its name promises. ModeRead is 0 and ModeWrite is 1, so the old
// "mode == ModeRead+ModeWrite" guard was really "mode == ModeWrite": the read
// half of setDeadline ran for ModeWrite too, and SetWriteDeadline silently
// overwrote the read deadline.
func TestDeadlineSettersAreIndependent(t *testing.T) {
	const tol = 500 * time.Millisecond

	t.Run("write deadline leaves read deadline alone", func(t *testing.T) {
		s := connectedPair(t)
		s.SetReadDeadline(time.Now().Add(1 * time.Hour))
		s.SetWriteDeadline(time.Now().Add(3 * time.Hour))
		rd, wd := deadlines(s)
		if !approx(rd, 1*time.Hour, tol) {
			t.Fatalf("read deadline is %v after SetWriteDeadline, want ~1h: the write setter clobbered it", time.Duration(rd))
		}
		if !approx(wd, 3*time.Hour, tol) {
			t.Fatalf("write deadline is %v, want ~3h", time.Duration(wd))
		}
	})

	t.Run("read deadline leaves write deadline alone", func(t *testing.T) {
		s := connectedPair(t)
		s.SetWriteDeadline(time.Now().Add(3 * time.Hour))
		s.SetReadDeadline(time.Now().Add(1 * time.Hour))
		rd, wd := deadlines(s)
		if !approx(rd, 1*time.Hour, tol) {
			t.Fatalf("read deadline is %v, want ~1h", time.Duration(rd))
		}
		if !approx(wd, 3*time.Hour, tol) {
			t.Fatalf("write deadline is %v after SetReadDeadline, want ~3h: the read setter clobbered it", time.Duration(wd))
		}
	})

	t.Run("SetDeadline sets both", func(t *testing.T) {
		s := connectedPair(t)
		s.SetDeadline(time.Now().Add(2 * time.Hour))
		rd, wd := deadlines(s)
		if !approx(rd, 2*time.Hour, tol) {
			t.Fatalf("read deadline is %v, want ~2h", time.Duration(rd))
		}
		if !approx(wd, 2*time.Hour, tol) {
			t.Fatalf("write deadline is %v, want ~2h", time.Duration(wd))
		}
	})

	t.Run("clearing the write deadline leaves the read deadline armed", func(t *testing.T) {
		s := connectedPair(t)
		s.SetReadDeadline(time.Now().Add(1 * time.Hour))
		s.SetWriteDeadline(time.Time{})
		rd, _ := deadlines(s)
		if !approx(rd, 1*time.Hour, tol) {
			t.Fatalf("read deadline is %v after clearing the write deadline, want ~1h", time.Duration(rd))
		}
	})
}

// The observable consequence of the aliasing bug: an armed read deadline
// survives a later SetWriteDeadline, so Read still times out on schedule
// instead of inheriting the (much longer) write deadline.
func TestSetWriteDeadlineDoesNotExtendRead(t *testing.T) {
	s := connectedPair(t)

	s.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	s.SetWriteDeadline(time.Now().Add(30 * time.Second))

	d, err := readAfter(t, s)
	t.Logf("Read returned after %v with err=%v", d.Round(time.Millisecond), err)

	if _, ok := err.(*SrtEpollTimeout); !ok {
		t.Fatalf("expected *SrtEpollTimeout, got %T: %v", err, err)
	}
	if d > 2*time.Second {
		t.Fatalf("Read returned after %v, want ~300ms: SetWriteDeadline reset the read deadline", d)
	}
}
