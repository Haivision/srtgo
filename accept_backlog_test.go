package srtgo

import (
	"testing"
	"time"
)

// Sockets are registered edge-triggered, and libsrt collapses every connection
// arriving between two srt_epoll_uwait calls into a single SRT_EPOLL_IN notice
// which it clears once reported. A backlog therefore yields exactly one
// readiness edge however many connections it holds, and no further edge is
// raised while the readiness bit stays set.
//
// Accept() used to wait for an edge before every accept, so it took one
// connection from a burst and then waited forever for an edge that was never
// coming -- losing the rest permanently, and going deaf to later arrivals too.
// Reported as issue #79.
func TestAcceptDrainsBacklog(t *testing.T) {
	const clients = 20

	port := randomPort()
	opts := map[string]string{"blocking": "0", "transtype": "file"}
	ln := NewSrtSocket("127.0.0.1", port, opts)
	if ln == nil {
		t.Fatal("could not create listener")
	}
	defer ln.Close()
	//Backlog has to exceed the burst, or libsrt legitimately refuses the
	//surplus and the test measures the backlog limit instead of the bug.
	if err := ln.Listen(clients * 2); err != nil {
		t.Fatal("listen:", err)
	}

	stop := make(chan struct{})
	defer close(stop)
	for i := 0; i < clients; i++ {
		go func() {
			c := NewSrtSocket("127.0.0.1", port, opts)
			if c == nil {
				return
			}
			defer c.Close()
			if err := c.Connect(); err != nil {
				return
			}
			<-stop
		}()
	}

	//Let the whole burst land in the backlog before Accept() is ever called,
	//so every connection but one arrives with no reader waiting on an edge.
	time.Sleep(1500 * time.Millisecond)

	accepted := 0
	for i := 0; i < clients; i++ {
		got := make(chan bool, 1)
		go func() {
			ln.SetReadDeadline(time.Now().Add(2 * time.Second))
			sock, _, err := ln.Accept()
			if err != nil || sock == nil {
				got <- false
				return
			}
			sock.Close()
			got <- true
		}()
		if !<-got {
			break
		}
		accepted++
	}

	if accepted != clients {
		t.Fatalf("accepted %d of %d queued connections; the rest are stranded in the backlog", accepted, clients)
	}
}
