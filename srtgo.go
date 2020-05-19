package srtgo

/*
#cgo LDFLAGS: -lsrt
#include <srt/srt.h>
static const SRTSOCKET get_srt_invalid_sock() { return SRT_INVALID_SOCK; };
static const int get_srt_error() { return SRT_ERROR; };
*/
import "C"

import (
	"fmt"
	"log"
	"strconv"
	"syscall"
	"unsafe"
)

// SRT Socket mode
const (
	ModeFailure = iota
	ModeListener
	ModeCaller
	ModeRendezvouz
)

// Binding ops
const (
	bindingPre  = 0
	bindingPost = 1
)

// SRT socket
type SrtSocket struct {
	socket       C.int
	epollConnect C.int
	epollIo      C.int
	blocking     bool
	host         string
	port         uint16
	options      map[string]string
	mode         int
	pktSize      int
}

//Static consts from library
var (
	SRT_INVALID_SOCK = C.get_srt_invalid_sock()
	SRT_ERROR        = C.get_srt_error()
)

const defaultPacketSize = 1456

// Initialize srt library
func InitSRT() {
	C.srt_startup()
}

// Cleanup SRT lib
func CleanupSRT() {
	C.srt_cleanup()
}

// Create a new SRT Socket
func NewSrtSocket(host string, port uint16, options map[string]string) *SrtSocket {
	s := new(SrtSocket)

	s.socket = C.srt_create_socket()
	if s.socket == SRT_INVALID_SOCK {
		return nil
	}

	s.host = host
	s.port = port
	s.options = options

	val, exists := options["pktsize"]
	if exists {
		pktSize, err := strconv.Atoi(val)
		if err != nil {
			s.pktSize = pktSize
		}
	}
	if s.pktSize <= 0 {
		s.pktSize = defaultPacketSize
	}

	val, s.blocking = options["blocking"]
	if !s.blocking || val == "0" {
		s.epollConnect = C.srt_epoll_create()
		if s.epollConnect < 0 {
			return nil
		}
		var modes C.int
		modes = C.SRT_EPOLL_OUT | C.SRT_EPOLL_ERR
		if C.srt_epoll_add_usock(s.epollConnect, s.socket, &modes) == SRT_ERROR {
			return nil
		}

		s.epollIo = C.srt_epoll_create()
		modes = C.SRT_EPOLL_IN | C.SRT_EPOLL_OUT | C.SRT_EPOLL_ERR
		if C.srt_epoll_add_usock(s.epollIo, s.socket, &modes) == SRT_ERROR {
			return nil
		}
	}

	var err error
	s.mode, err = s.preconfiguration()
	if err != nil {
		return nil
	}

	return s
}

func newFromSocket(acceptSocket *SrtSocket, socket C.SRTSOCKET) *SrtSocket {
	s := new(SrtSocket)
	s.socket = socket
	s.pktSize = acceptSocket.pktSize
	s.blocking = acceptSocket.blocking

	err := acceptSocket.postconfiguration(s)
	if err != nil {
		return nil
	}

	if !s.blocking {
		s.epollIo = C.srt_epoll_create()
		var modes C.int = C.SRT_EPOLL_IN | C.SRT_EPOLL_OUT | C.SRT_EPOLL_ERR
		if C.srt_epoll_add_usock(s.epollIo, s.socket, &modes) == SRT_ERROR {
			return nil
		}
	}

	return s
}

// Start listening for incoming connections
func (s SrtSocket) Listen(clients int) error {
	nclients := C.int(clients)

	sa, salen, err := CreateAddrInet(s.host, s.port)
	if err != nil {
		return err
	}

	res := C.srt_bind(s.socket, sa, C.int(salen))
	if res == SRT_ERROR {
		C.srt_close(s.socket)
		return fmt.Errorf("Error in srt_bind")
	}

	res = C.srt_listen(s.socket, nclients)
	if res == SRT_ERROR {
		C.srt_close(s.socket)
		return fmt.Errorf("Error in srt_listen")
	}

	err = s.postconfiguration(&s)
	if err != nil {
		return fmt.Errorf("Error setting post socket options")
	}

	return nil
}

// Accept an incoming connection
func (s SrtSocket) Accept() (*SrtSocket, error) {
	if !s.blocking {
		// Socket readiness for connection is checked by polling on WRITE allowed sockets.
		len := C.int(2)
		timeoutMs := C.long(-1)
		ready := [2]C.int{SRT_INVALID_SOCK, SRT_INVALID_SOCK}
		if C.srt_epoll_wait(s.epollConnect, nil, nil, &ready[0], &len, timeoutMs, nil, nil, nil, nil) == -1 {
			return nil, fmt.Errorf("srt accept, epoll wait issue")
		}
	}

	var scl syscall.RawSockaddrInet4
	sclen := C.int(syscall.SizeofSockaddrInet4)
	socket := C.srt_accept(s.socket, (*C.struct_sockaddr)(unsafe.Pointer(&scl)), &sclen)
	if socket == SRT_INVALID_SOCK {
		return nil, fmt.Errorf("srt accept, error accepting the connection")
	}

	log.Println("Connection accepted!")

	newSocket := newFromSocket(&s, socket)
	if newSocket == nil {
		return nil, fmt.Errorf("new socket could not be created")
	}

	return newSocket, nil
}

// Connect to a remote endpoint
func (s SrtSocket) Connect() error {
	sa, salen, err := CreateAddrInet(s.host, s.port)
	if err != nil {
		return err
	}

	res := C.srt_connect(s.socket, sa, C.int(salen))
	if res == SRT_ERROR {
		C.srt_close(s.socket)
		return fmt.Errorf("Error in srt_connect")
	}

	if !s.blocking {
		// Socket readiness for connection is checked by polling on WRITE allowed sockets.
		len := C.int(2)
		timeoutMs := C.long(-1)
		ready := [2]C.int{SRT_INVALID_SOCK, SRT_INVALID_SOCK}
		if C.srt_epoll_wait(s.epollConnect, nil, nil, &ready[0], &len, timeoutMs, nil, nil, nil, nil) != -1 {
			state := C.srt_getsockstate(s.socket)
			if state != C.SRTS_CONNECTED {
				return fmt.Errorf("srt connect, connection failed %d", state)
			}
		} else {
			return fmt.Errorf("srt connect, epoll wait issue")
		}
	}

	err = s.postconfiguration(&s)
	if err != nil {
		return fmt.Errorf("Error setting post socket options in connect")
	}

	return nil
}

// Read data from the SRT socket
func (s SrtSocket) Read(b []byte, timeout int) (n int, err error) {
	if !s.blocking {
		len := C.int(2)
		timeoutMs := C.long(timeout)
		ready := [2]C.int{SRT_INVALID_SOCK, SRT_INVALID_SOCK}

		if C.srt_epoll_wait(s.epollIo, &ready[0], &len, nil, nil, timeoutMs, nil, nil, nil, nil) == SRT_ERROR {
			if C.srt_getlasterror(nil) == C.SRT_ETIMEOUT {
				return 0, nil
			}
			return 0, fmt.Errorf("error in read:epoll")
		}
	}

	res := C.srt_recvmsg2(s.socket, (*C.char)(unsafe.Pointer(&b[0])), C.int(len(b)), nil)
	if res == SRT_ERROR {
		return 0, fmt.Errorf("error in read::recv %s", C.GoString(C.srt_getlasterror_str()))
	}

	return int(res), nil
}

// Write data to the SRT socket
func (s SrtSocket) Write(b []byte, timeout int) (n int, err error) {
	if !s.blocking {
		timeoutMs := C.long(timeout)
		len := C.int(2)
		ready := [2]C.int{SRT_INVALID_SOCK, SRT_INVALID_SOCK}
		rlen := C.int(2)
		rready := [2]C.int{SRT_INVALID_SOCK, SRT_INVALID_SOCK}

		if C.srt_epoll_wait(s.epollIo, &rready[0], &rlen, &ready[0], &len, timeoutMs, nil, nil, nil, nil) == SRT_ERROR {
			return 0, fmt.Errorf("error in read:epoll")
		}
	}

	res := C.srt_sendmsg2(s.socket, (*C.char)(unsafe.Pointer(&b[0])), C.int(len(b)), nil)
	if res == SRT_ERROR {
		return 0, fmt.Errorf("error in read:srt_sendmsg2")
	}

	return int(res), nil
}

// Retrieve stats from the SRT socket
func (s SrtSocket) Stats() (*SrtStats, error) {
	var stats C.SRT_TRACEBSTATS = C.SRT_TRACEBSTATS{}
	var b C.int = 1
	if C.srt_bstats(s.socket, &stats, b) == SRT_ERROR {
		return nil, fmt.Errorf("Error getting stats")
	}

	return newSrtStats(&stats), nil
}

// Return working mode of the SRT socket
func (s SrtSocket) Mode() int {
	return s.mode
}

// Return packet size of the SRT socket
func (s SrtSocket) PacketSize() int {
	return s.pktSize
}

// Close the SRT socket
func (s SrtSocket) Close() {
	if !s.blocking {
		if s.epollConnect != -1 {
			C.srt_epoll_release(s.epollConnect)
			s.epollConnect = -1
		}
		if s.epollIo != -1 {
			C.srt_epoll_release(s.epollIo)
			s.epollIo = -1
		}
	}
	C.srt_close(s.socket)
	log.Println("Connection closed")
}

func (s SrtSocket) preconfiguration() (int, error) {
	var blocking C.int
	if s.blocking {
		blocking = C.int(1)
	} else {
		blocking = C.int(0)
	}
	result := C.srt_setsockopt(s.socket, 0, C.SRTO_RCVSYN, unsafe.Pointer(&blocking), C.int(unsafe.Sizeof(blocking)))
	if result == -1 {
		return ModeFailure, fmt.Errorf("Could not set SRTO_RCVSYN flag")
	}

	var mode int
	modeVal, ok := s.options["mode"]
	if !ok {
		modeVal = "default"
	}

	if modeVal == "client" || modeVal == "caller" {
		mode = ModeCaller
	} else if modeVal == "server" || modeVal == "listener" {
		mode = ModeListener
	} else if modeVal == "default" {
		if s.host == "" {
			mode = ModeListener
		} else {
			// Host is given, so check also "adapter"
			if _, ok := s.options["adapter"]; ok {
				mode = ModeRendezvouz
			} else {
				mode = ModeCaller
			}
		}
	} else {
		mode = ModeFailure
	}

	if linger, ok := s.options["linger"]; ok {
		li, err := strconv.Atoi(linger)
		if err == nil {
			setSocketLingerOption(s.socket, int32(li))
		} else {
			return ModeFailure, fmt.Errorf("Could not set LINGER option")
		}
	}

	err := setSocketOptions(s.socket, bindingPre, s.options)
	if err != nil {
		return ModeFailure, fmt.Errorf("Error setting socket options")
	}

	return mode, nil
}

func (s SrtSocket) postconfiguration(sck *SrtSocket) error {

	var blocking C.int
	if s.blocking {
		blocking = 1
	} else {
		blocking = 0
	}

	res := C.srt_setsockopt(sck.socket, 0, C.SRTO_SNDSYN, unsafe.Pointer(&blocking), C.int(unsafe.Sizeof(blocking)))
	if res == -1 {
		fmt.Errorf("Error in postconfiguration setting SRTO_SNDSYN")
	}

	res = C.srt_setsockopt(sck.socket, 0, C.SRTO_RCVSYN, unsafe.Pointer(&blocking), C.int(unsafe.Sizeof(blocking)))
	if res == -1 {
		fmt.Errorf("Error in postconfiguration setting SRTO_RCVSYN")
	}

	err := setSocketOptions(sck.socket, bindingPost, s.options)
	return err
}
