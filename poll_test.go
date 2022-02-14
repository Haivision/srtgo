package srtgo

import (
	"testing"
)

func connectLoop(port uint16, semChan chan struct{}) {
	for {
		//fmt.Printf("Connecting\n")
		s := NewSrtSocket("127.0.0.1", port, map[string]string{"blocking": "0", "mode": "caller"})
		err := s.Connect()
		if err != nil {
			continue
		}
		s.Close()
		_, ok := <-semChan
		if !ok {
			return
		}
	}
}

func benchAccept(blocking string, N int) {
start:
	port := randomPort()
	s := NewSrtSocket("127.0.0.1", port, map[string]string{"blocking": blocking, "mode": "listener"})
	if s == nil {
		goto start
	}
	if err := s.Listen(5); err != nil {
		goto start
	}
	semChan := make(chan struct{}, 8)
	go connectLoop(port, semChan)
	semChan <- struct{}{}
	semChan <- struct{}{}
	for i := 0; i < N; i++ {
		semChan <- struct{}{}
		_, _, _ = s.Accept()
	}
	close(semChan)
	s.Close()
}

func BenchmarkAcceptBlocking(b *testing.B) {
	benchAccept("1", b.N)
}

func BenchmarkAcceptNonBlocking(b *testing.B) {
	benchAccept("0", b.N)
}

/*
func BenchmarkAcceptNonBlockingParallel(b *testing.B) {
	SrtSetLogLevel(SrtLogLevelCrit)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			benchAccept("0", 2)
		}
	})
}

func BenchmarkAcceptBlockingParallel(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			benchAccept("1", 2)
		}
	})
}
*/
