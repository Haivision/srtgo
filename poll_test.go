package srtgo

import (
	"math/rand"
	"testing"
)

func BenchmarkAcceptNonBlocking(b *testing.B) {
	SrtSetLogLevel(SrtLogLevelCrit)
	port := uint16(rand.Uint32())
	s := NewSrtSocket("localhost", port, map[string]string{"blocking": "0", "mode": "listener"})
	if s == nil {
		panic("ja")
	}
	s.Listen(1)
	b.ResetTimer()
	go func() {
		for {
			s := NewSrtSocket("localhost", port, map[string]string{"blocking": "0", "mode": "caller"})
			s.Connect()
			s.Close()
		}
	}()
	for i := 0; i < b.N; i++ {
		_, _, _ = s.Accept()
	}
	s.Close()
}

func BenchmarkAcceptBlocking(b *testing.B) {
	SrtSetLogLevel(SrtLogLevelCrit)
	port := uint16(rand.Uint32())
	s := NewSrtSocket("localhost", port, map[string]string{"blocking": "1", "mode": "listener"})
	if s == nil {
		panic("ja")
	}
	s.Listen(1)
	b.ResetTimer()
	go func() {
		for {
			s := NewSrtSocket("localhost", port, map[string]string{"blocking": "1", "mode": "caller"})
			if s == nil {
				panic("NO SOCKET")
			}
			s.Connect()
			s.Close()
		}
	}()
	for i := 0; i < b.N; i++ {
		_, _, _ = s.Accept()
	}
	s.Close()
}

func BenchmarkAcceptNonBlockingParallel(b *testing.B) {
	SrtSetLogLevel(SrtLogLevelCrit)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
		portselect:
			port := uint16(rand.Uint32())
			s := NewSrtSocket("localhost", port, map[string]string{"blocking": "0", "mode": "listener"})
			if err := s.Listen(1); err != nil {
				goto portselect
			}
			go func() {
				for {
					s := NewSrtSocket("localhost", port, map[string]string{"blocking": "0", "mode": "caller"})
					if s == nil {
						panic("SOCKET IS NIL")
					}
					s.Connect()
					s.Close()
				}
			}()
			for i := 0; i < 8; i++ {
				_, _, _ = s.Accept()
			}
			s.Close()
		}
	})
}

func BenchmarkAcceptBlockingParallel(b *testing.B) {
	SrtSetLogLevel(SrtLogLevelCrit)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
		portselect:
			port := uint16(rand.Uint32())
			s := NewSrtSocket("localhost", port, map[string]string{"blocking": "1", "mode": "listener"})
			if err := s.Listen(1); err != nil {
				goto portselect
			}

			go func() {
				for {
					s := NewSrtSocket("localhost", port, map[string]string{"blocking": "0", "mode": "caller"})
					s.Connect()
					s.Close()
				}
			}()
			for i := 0; i < 8; i++ {
				_, _, _ = s.Accept()
			}
			s.Close()
		}
	})
}
