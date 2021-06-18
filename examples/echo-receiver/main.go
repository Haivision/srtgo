package main

// #cgo LDFLAGS: -lsrt
// #include <srt/srt.h>
import "C"

import (
	"log"
	"net"

	"github.com/haivision/srtgo"
)

var allowedStreamIDs = map[string]bool{
	"foo":    true,
	"foobar": true,
}

func listenCallback(socket *srtgo.SrtSocket, version int, addr *net.UDPAddr, streamid string) bool {
	log.Printf("socket will connect, hsVersion: %d, streamid: %s\n", version, streamid)

	// socket not in allowed ids -> reject
	if _, found := allowedStreamIDs[streamid]; !found {
		// set custom reject reason
		socket.SetRejectReason(srtgo.RejectionReasonUnauthorized)
		return false
	}

	// allow connection
	return true
}

// echo received packets
func handler(socket *srtgo.SrtSocket, addr *net.UDPAddr) {
	buf := make([]byte, 1500)
	for {
		len, err := socket.Read(buf, 1)
		if err != nil {
			log.Println(err)
			return
		}
		_, err = socket.Write(buf[:len], 1)
		if err != nil {
			log.Println(err)
			return
		}
	}
}

func main() {
	// set socket options
	options := make(map[string]string)
	options["blocking"] = "0"
	options["transtype"] = "live"
	options["latency"] = "300"

	// create and bind socket
	sck := srtgo.NewSrtSocket("127.0.0.1", 6000, options)

	// set listen callback
	sck.SetListenCallback(listenCallback)

	// start listenening
	err := sck.Listen(1)
	log.Println("started listening on port 6000")
	if err != nil {
		log.Fatalf("Listen failed: %v \n", err.Error())
	}

	for {
		// accept client sockets
		socket, peeraddr, err := sck.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go handler(socket, peeraddr)
	}
}
