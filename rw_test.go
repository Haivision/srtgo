package srtgo

import (
	"context"
	"fmt"
	"sync"
	"testing"
)

type tester struct {
	blocking string          // blocking state
	ctx      context.Context // done notification for senders
	cancel   func()          // cancel tester
	err      chan error      // error output for senders/receivers
	started  sync.WaitGroup  // sync goroutines on start
}

// Creates a connector socket and keeps writing to it
func (t *tester) send(port uint16) {
	buf := make([]byte, 1316)
	sock := NewSrtSocket("127.0.0.1", port, map[string]string{"blocking": t.blocking, "mode": "caller", "transtype": "file"})
	if sock == nil {
		t.err <- fmt.Errorf("failed to create caller socket")
		return
	}
	defer sock.Close()

	// Wait until all sockets are ready
	t.started.Done()
	t.started.Wait()

	err := sock.Connect()
	if err != nil {
		t.err <- fmt.Errorf("connect: %v", err)
		return
	}
	for {
		select {
		case <-t.ctx.Done():
			return
		default:
		}
		_, err := sock.Write(buf)
		if err != nil {
			t.err <- fmt.Errorf("write: %v", err)
			return
		}
	}
}

// Creates a listener socket and keeps reading from it
func (t *tester) receive(port uint16, iterations int) {
	sock := NewSrtSocket("127.0.0.1", port, map[string]string{"blocking": t.blocking, "mode": "listener", "rcvbuf": "22937600", "transtype": "file"})
	if sock == nil {
		t.err <- fmt.Errorf("failed to create listener socket")
		return
	}
	buf := make([]byte, 1316)
	defer sock.Close()
	defer t.cancel()

	// Wait until all sockets are ready
	t.started.Done()
	t.started.Wait()

	err := sock.Listen(1)
	if err != nil {
		t.err <- fmt.Errorf("listen: %v", err)
		return
	}
	remote, _, err := sock.Accept()
	if err != nil {
		t.err <- fmt.Errorf("accept: %v", err)
		return
	}
	for i := 0; i < iterations; i++ {
		_, err := remote.Read(buf)
		if err != nil {
			t.err <- fmt.Errorf("read: %v", err)
			return
		}
	}
}

func runTransmitBench(b *testing.B, blocking bool) {
	blockStr := "0"
	if blocking {
		blockStr = "1"
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	t := &tester{
		ctx:      ctx,
		blocking: blockStr,
		cancel:   cancel,
		err:      make(chan error, 1),
	}
	t.started.Add(2)
	port := randomPort()
	go t.receive(port, b.N)
	go t.send(port)

	t.started.Wait()
	b.ResetTimer()
	select {
	case <-ctx.Done():
		return
	case err := <-t.err:
		b.Error(err)
	}
}

func BenchmarkRWBlocking(b *testing.B) {
	runTransmitBench(b, true)
}

func BenchmarkRWNonBlocking(b *testing.B) {
	runTransmitBench(b, false)
}
