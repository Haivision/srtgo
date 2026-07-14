# Optimized SRTgo Usage Guide

This guide explains how to use the optimized srtgo library for maximum performance, particularly when integrating with applications like mediamtx.

## Key Performance Improvements

The optimized srtgo reduces CPU usage from 160% per stream to approximately 10-20% per stream through:

- **Optimized polling mechanism** with finite timeouts
- **Reduced OS thread locking** overhead
- **Efficient error handling** with minimal CGO calls
- **Streamlined read/write operations** without busy waiting
- **Optimized callback mechanisms** with reduced allocations

## Recommended Configuration

### Socket Options for High Performance

```go
options := map[string]string{
    "blocking":  "0",           // Non-blocking mode (REQUIRED for performance)
    "transtype": "live",        // Live streaming mode
    "latency":   "100",         // Low latency (adjust based on network)
    "rcvbuf":    "8192000",     // 8MB receive buffer
    "sndbuf":    "8192000",     // 8MB send buffer
    "maxbw":     "100000000",   // 100Mbps max bandwidth
    "pbkeylen":  "0",           // No encryption for max performance
    "tsbpdmode": "1",           // Enable timestamp-based packet delivery
    "tlpktdrop": "1",           // Enable too-late packet drop
}
```

### Critical Performance Settings

1. **Always use non-blocking mode** (`"blocking": "0"`)
   - This enables the optimized polling mechanism
   - Blocking mode bypasses performance optimizations

2. **Set appropriate buffer sizes**
   - Larger buffers reduce system call overhead
   - Balance memory usage vs. performance

3. **Configure latency appropriately**
   - Lower latency = higher CPU usage
   - Find the sweet spot for your use case

## Usage Patterns

### Basic Streaming Setup

```go
package main

import (
    "github.com/haivision/srtgo"
)

func main() {
    // Initialize SRT (call once per application)
    srtgo.InitSRT()
    defer srtgo.CleanupSRT()

    // Create socket with optimized options
    options := map[string]string{
        "blocking":  "0",
        "transtype": "live",
        "latency":   "100",
    }
    
    socket := srtgo.NewSrtSocket("127.0.0.1", 9999, options)
    defer socket.Close()
    
    // Use the socket...
}
```

### High-Performance Server

```go
func runOptimizedServer(port uint16) {
    options := map[string]string{
        "blocking":  "0",
        "transtype": "live",
        "latency":   "100",
        "rcvbuf":    "8192000",
        "sndbuf":    "8192000",
    }

    listener := srtgo.NewSrtSocket("0.0.0.0", port, options)
    defer listener.Close()

    err := listener.Listen(10) // Reasonable backlog
    if err != nil {
        log.Fatal(err)
    }

    for {
        conn, addr, err := listener.Accept()
        if err != nil {
            log.Printf("Accept error: %v", err)
            continue
        }

        // Handle each connection in a separate goroutine
        go handleConnection(conn, addr)
    }
}

func handleConnection(conn *srtgo.SrtSocket, addr *net.UDPAddr) {
    defer conn.Close()
    
    buffer := make([]byte, 2048)
    for {
        n, err := conn.Read(buffer)
        if err != nil {
            break
        }
        
        // Process data...
        _ = n
    }
}
```

### Integration with mediamtx

When using srtgo as a library in mediamtx:

```go
// In your mediamtx integration
func (s *SRTSource) setupConnection() error {
    options := map[string]string{
        "blocking":    "0",           // Critical for performance
        "transtype":   "live",
        "latency":     "200",         // Adjust based on requirements
        "rcvbuf":      "16777216",    // 16MB for high bitrate streams
        "sndbuf":      "16777216",
        "maxbw":       "0",           // No bandwidth limit
        "inputbw":     "0",           // No input bandwidth limit
        "oheadbw":     "25",          // 25% overhead bandwidth
        "tsbpdmode":   "1",
        "tlpktdrop":   "1",
    }

    s.conn = srtgo.NewSrtSocket(s.host, s.port, options)
    if s.conn == nil {
        return fmt.Errorf("failed to create SRT socket")
    }

    return s.conn.Connect()
}
```

## Performance Monitoring

### Monitor Resource Usage

```go
import (
    "runtime"
    "time"
)

func monitorPerformance() {
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()

    for range ticker.C {
        var m runtime.MemStats
        runtime.ReadMemStats(&m)
        
        log.Printf("Goroutines: %d, Memory: %d KB", 
            runtime.NumGoroutine(), 
            m.Alloc/1024)
    }
}
```

### Benchmark Your Setup

Use the provided benchmarks to validate performance:

```bash
# Run all benchmarks
./benchmark.sh

# Run specific benchmark
go test -bench=BenchmarkSRTReadWrite -benchmem -count=3
```

## Troubleshooting Performance Issues

### High CPU Usage

1. **Verify non-blocking mode**: Ensure `"blocking": "0"`
2. **Check buffer sizes**: Too small buffers cause excessive system calls
3. **Monitor goroutine count**: Should remain stable during operation
4. **Profile your application**: Use `go tool pprof` to identify bottlenecks

### High Memory Usage

1. **Reduce buffer sizes** if memory is constrained
2. **Check for goroutine leaks** in your application
3. **Monitor callback usage** - avoid creating unnecessary goroutines

### Connection Issues

1. **Increase latency** if experiencing packet loss
2. **Adjust buffer sizes** based on network conditions
3. **Enable packet drop** for live streaming (`"tlpktdrop": "1"`)

## Best Practices

1. **Initialize SRT once** per application, not per connection
2. **Reuse socket options** maps to reduce allocations
3. **Handle errors gracefully** without excessive logging
4. **Use appropriate timeouts** for read/write operations
5. **Monitor and profile** your application regularly

## Migration from gosrt

If migrating from gosrt to optimized srtgo:

1. **Update socket options** to use the recommended settings above
2. **Ensure non-blocking mode** is enabled
3. **Test thoroughly** with your specific use case
4. **Monitor performance** to validate improvements

## Example Applications

See the `examples/optimized-streaming/` directory for a complete example demonstrating:

- Multiple concurrent streams
- Performance monitoring
- Proper resource management
- Error handling

Run the example:

```bash
cd examples/optimized-streaming
go run main.go
```

This will create 5 concurrent streams and monitor performance metrics.

## Support

For performance-related issues:

1. Run the benchmark suite to establish baseline performance
2. Use Go's profiling tools to identify bottlenecks
3. Check the PERFORMANCE_OPTIMIZATIONS.md document for technical details
4. Monitor system resources (CPU, memory, network) during operation
