package srtgo

import (
	"os"
	"testing"
)

// TestMain shuts the SRT library down before the test binary exits.
//
// Without this the process segfaults on exit on Linux, inside libsrt's own
// receive-queue worker:
//
//	Thread 35 "SRT:RcvQ:w13" received signal SIGSEGV
//	#0  srt::CRcvQueue::worker(void*) () from libsrt.so.1.5
//
// Any bound socket spawns an RcvQ worker thread, and srt_close() only starts
// an asynchronous teardown -- SRT reaps the multiplexer about a second later.
// If the process exits in the meantime, libsrt's global destructors free state
// out from under a thread that is still running. Tests that never close their
// sockets crash every time; tests that do close still lose the race sometimes.
//
// srt_cleanup() shuts those threads down first, which is exactly what it is
// for. Note it is reference counted against srt_startup(): do NOT add an
// InitSRT() call here, or the count never reaches zero and the cleanup is a
// no-op. Individual tests already call InitSRT() as needed.
func TestMain(m *testing.M) {
	code := m.Run()
	CleanupSRT()
	os.Exit(code)
}
