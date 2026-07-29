package srtgo

import (
	"os"
	"testing"
)

// TestMain shuts SRT down before the test binary exits.
//
// Without it the process segfaults on exit, inside libsrt's own receive-queue
// worker rather than in Go:
//
//	Thread 35 "SRT:RcvQ:w13" received signal SIGSEGV
//	#0  srt::CRcvQueue::worker(void*) () from libsrt.so.1.5
//
// Any bound socket spawns an RcvQ worker thread, and srt_close() only starts
// an asynchronous teardown -- SRT reaps the multiplexer about a second later.
// Exiting in the meantime lets libsrt's global destructors free state under a
// thread that is still running.
//
// Do NOT add an InitSRT() call here: srt_cleanup is reference counted against
// srt_startup, so the extra startup leaves the count above zero and the
// cleanup silently does nothing.
func TestMain(m *testing.M) {
	code := m.Run()
	CleanupSRT()
	os.Exit(code)
}
