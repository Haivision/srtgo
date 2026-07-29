[![CI](https://github.com/Haivision/srtgo/actions/workflows/ci.yml/badge.svg?branch=master)](https://github.com/Haivision/srtgo/actions/workflows/ci.yml)
[![PkgGoDev](https://pkg.go.dev/badge/github.com/haivision/srtgo)](https://pkg.go.dev/github.com/haivision/srtgo)

# srtgo

Go bindings for [SRT](https://github.com/Haivision/srt) (Secure Reliable Transport), the open source transport technology that optimizes streaming performance across unpredictable networks.

## Why srtgo?
The purpose of srtgo is easing the adoption of SRT transport technology. Using Go, with just a few lines of code you can implement an application that sends/receives data with all the benefits of SRT technology: security and reliability, while keeping latency low.

## Is this a new implementation of SRT?
No! We are just exposing the great work done by the community in the [SRT project](https://github.com/Haivision/srt) as a golang library. All the functionality and implementation still resides in the official SRT project.


# Features supported
* Basic API exposed to easy develop SRT sender/receiver apps
* Caller and Listener mode
* Live transport type
* File transport type
* Message/Buffer API
* SRT transport options up to SRT 1.4.1 (options added by later libsrt releases are not exposed yet)
* SRT Stats retrieval

# Usage
Example of a SRT receiver application:
``` go
package main

import (
    "github.com/haivision/srtgo"
    "fmt"
)

func main() {
    options := make(map[string]string)
    options["transtype"] = "file"

    sck := srtgo.NewSrtSocket("0.0.0.0", 8090, options)
    defer sck.Close()
    sck.Listen(1)
    s, _ := sck.Accept()
    defer s.Close()

    buf := make([]byte, 2048)
    for {
        n, _ := s.Read(buf)
        if n == 0 {
            break
        }
        fmt.Println("Received %d bytes", n)
    }
    //....
}

```


# Dependencies

* srtlib

You can find detailed instructions about how to install srtlib in its [README file](https://github.com/Haivision/srt#requirements)

srtgo requires **srt 1.4.2 or newer** to build: it uses APIs first introduced in 1.4.2 (`srt_connect_callback`, `srt_setrejectreason`, and the `SRT_EPOLLEMPTY`/`SRT_ESCLOSED`/`SRT_ESYSOBJ` error codes).

For security, **srt 1.5.6 or newer is recommended**: every earlier release is affected by CVE-2026-55869 (buffer overflow in KMREQ/KMRSP handling) and CVE-2026-55868 (encryption state machine downgrade), both fixed in [srt 1.5.6](https://github.com/Haivision/srt/releases/tag/v1.5.6). See [SECURITY.md](SECURITY.md).
