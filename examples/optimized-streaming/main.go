package main

import (
	"context"
	"log"
	"runtime"
	"sync"
	"time"

	"github.com/haivision/srtgo"
)

func main() {
	// Initialize SRT
	srtgo.InitSRT()
	defer srtgo.CleanupSRT()

	log.Println("Starting optimized SRT streaming example...")
	log.Printf("Initial goroutines: %d", runtime.NumGoroutine())

	// Create multiple concurrent streams to demonstrate performance
	const numStreams = 5
	const streamDuration = 10 * time.Second

	ctx, cancel := context.WithTimeout(context.Background(), streamDuration)
	defer cancel()

	var wg sync.WaitGroup

	// Start multiple streaming pairs
	for i := 0; i < numStreams; i++ {
		wg.Add(1)
		go func(streamID int) {
			defer wg.Done()
			runStreamPair(ctx, streamID)
		}(i)
	}

	// Monitor goroutine count
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				log.Printf("Active goroutines: %d", runtime.NumGoroutine())
			}
		}
	}()

	wg.Wait()
	log.Printf("Final goroutines: %d", runtime.NumGoroutine())
	log.Println("Streaming test completed successfully!")
}

func runStreamPair(ctx context.Context, streamID int) {
	port := uint16(30000 + streamID)

	// Optimized options for low latency and high performance
	options := map[string]string{
		"blocking":  "0",           // Non-blocking mode for better performance
		"transtype": "live",        // Live streaming mode
		"latency":   "100",         // Low latency (100ms)
		"rcvbuf":    "8192000",     // 8MB receive buffer
		"sndbuf":    "8192000",     // 8MB send buffer
		"maxbw":     "100000000",   // 100Mbps max bandwidth
		"pbkeylen":  "0",           // No encryption for performance
	}

	// Create listener
	listener := srtgo.NewSrtSocket("127.0.0.1", port, options)
	if listener == nil {
		log.Printf("Stream %d: Failed to create listener", streamID)
		return
	}
	defer listener.Close()

	err := listener.Listen(1)
	if err != nil {
		log.Printf("Stream %d: Failed to listen: %v", streamID, err)
		return
	}

	// Create client
	client := srtgo.NewSrtSocket("127.0.0.1", port, options)
	if client == nil {
		log.Printf("Stream %d: Failed to create client", streamID)
		return
	}
	defer client.Close()

	// Connect in background
	var connectWg sync.WaitGroup
	connectWg.Add(1)
	go func() {
		defer connectWg.Done()
		err := client.Connect()
		if err != nil {
			log.Printf("Stream %d: Client connect failed: %v", streamID, err)
		}
	}()

	// Accept connection
	server, _, err := listener.Accept()
	if err != nil {
		log.Printf("Stream %d: Failed to accept: %v", streamID, err)
		return
	}
	defer server.Close()

	connectWg.Wait()

	log.Printf("Stream %d: Connection established", streamID)

	// Start data transfer
	var transferWg sync.WaitGroup

	// Sender goroutine
	transferWg.Add(1)
	go func() {
		defer transferWg.Done()
		sendData(ctx, client, streamID)
	}()

	// Receiver goroutine
	transferWg.Add(1)
	go func() {
		defer transferWg.Done()
		receiveData(ctx, server, streamID)
	}()

	transferWg.Wait()
	log.Printf("Stream %d: Transfer completed", streamID)
}

func sendData(ctx context.Context, client *srtgo.SrtSocket, streamID int) {
	// Use realistic packet size for video streaming
	data := make([]byte, 1316) // Standard SRT packet size
	for i := range data {
		data[i] = byte((i + streamID) % 256)
	}

	packetCount := 0
	ticker := time.NewTicker(10 * time.Millisecond) // 100 packets/second
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("Stream %d: Sent %d packets", streamID, packetCount)
			return
		case <-ticker.C:
			_, err := client.Write(data)
			if err != nil {
				log.Printf("Stream %d: Send error: %v", streamID, err)
				return
			}
			packetCount++
		}
	}
}

func receiveData(ctx context.Context, server *srtgo.SrtSocket, streamID int) {
	buffer := make([]byte, 2048)
	packetCount := 0

	// Use a longer timeout and smaller check interval for better reception
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Give a small grace period to receive remaining packets
			gracePeriod := time.NewTimer(50 * time.Millisecond)
			for {
				select {
				case <-gracePeriod.C:
					log.Printf("Stream %d: Received %d packets", streamID, packetCount)
					return
				default:
					n, err := server.Read(buffer)
					if err != nil {
						if netErr, ok := err.(interface{ Timeout() bool }); ok && netErr.Timeout() {
							continue
						}
						log.Printf("Stream %d: Received %d packets", streamID, packetCount)
						return
					}
					if n > 0 {
						packetCount++
					}
				}
			}
		case <-ticker.C:
			// More frequent read attempts
			server.SetReadDeadline(time.Now().Add(50 * time.Millisecond))

			n, err := server.Read(buffer)
			if err != nil {
				// Check if it's a timeout (expected in this test)
				if netErr, ok := err.(interface{ Timeout() bool }); ok && netErr.Timeout() {
					continue
				}
				log.Printf("Stream %d: Receive error: %v", streamID, err)
				return
			}

			if n > 0 {
				packetCount++
			}
		}
	}
}
