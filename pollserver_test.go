package srtgo

import "testing"

// CleanupSRT stops the process-wide poll server. A consumer that shuts SRT
// down and later starts it again must get a working poll server back.
//
// When shutdown was first added, the poll server was created behind a
// sync.Once, so it could never be replaced: after CleanupSRT the next
// non-blocking socket tried to register against an epoll that had already
// been released, and pollOpen panicked with "ERROR ADDING FD TO EPOLL".
func TestPollServerRestartsAfterCleanup(t *testing.T) {
	InitSRT()
	opts := map[string]string{"blocking": "0", "transtype": "file"}

	first := NewSrtSocket("127.0.0.1", randomPort(), opts)
	if first == nil {
		t.Fatal("could not create the first socket")
	}
	first.Close()

	CleanupSRT()

	InitSRT()
	second := NewSrtSocket("127.0.0.1", randomPort(), opts)
	if second == nil {
		t.Fatal("could not create a socket after CleanupSRT")
	}
	defer second.Close()

	//Listen exercises the poll server: a non-blocking socket has to be
	//registered with the epoll for this to work at all.
	if err := second.Listen(1); err != nil {
		t.Fatalf("listen after CleanupSRT/InitSRT: %v", err)
	}
}
