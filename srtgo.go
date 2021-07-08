package srtgo

/*
#cgo LDFLAGS: -lsrt
#include <srt/srt.h>
#include <srt/access_control.h>
static const SRTSOCKET get_srt_invalid_sock() { return SRT_INVALID_SOCK; };
static const int get_srt_error() { return SRT_ERROR; };
static const int get_srt_error_reject_predefined() { return SRT_REJC_PREDEFINED; };
static const int get_srt_error_reject_userdefined() { return SRT_REJC_USERDEFINED; };

extern int srtListenCB(void* opaque, SRTSOCKET ns, int hs_version, const struct sockaddr* peeraddr, const char* streamid);
*/
import "C"

import (
	"errors"
	"fmt"
	"net"
	"runtime"
	"strconv"
	"sync"
	"syscall"
	"time"
	"unsafe"

	gopointer "github.com/mattn/go-pointer"
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

// SrtSocket - SRT socket
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
	pollTimeout  int64
}

var (
	listenCallbackMutex sync.Mutex
	listenCallbackMap   map[C.int]unsafe.Pointer = make(map[C.int]unsafe.Pointer)
)

// Static consts from library
var (
	SRT_INVALID_SOCK = C.get_srt_invalid_sock()
	SRT_ERROR        = C.get_srt_error()
	SRTS_CONNECTED   = C.SRTS_CONNECTED
)

const defaultPacketSize = 1456

// InitSRT - Initialize srt library
func InitSRT() {
	C.srt_startup()
}

// CleanupSRT - Cleanup SRT lib
func CleanupSRT() {
	C.srt_cleanup()
}

// NewSrtSocket - Create a new SRT Socket
func NewSrtSocket(host string, port uint16, options map[string]string) *SrtSocket {
	s := new(SrtSocket)

	s.socket = C.srt_create_socket()
	if s.socket == SRT_INVALID_SOCK {
		return nil
	}

	s.host = host
	s.port = port
	s.options = options
	s.pollTimeout = -1

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

	val, exists = options["blocking"]
	if exists && val != "0" {
		s.blocking = true
	}
	if !s.blocking {
		s.epollConnect = C.srt_epoll_create()
		if s.epollConnect < 0 {
			return nil
		}
		var modes C.int
		modes = C.SRT_EPOLL_IN | C.SRT_EPOLL_OUT | C.SRT_EPOLL_ERR
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

func newFromSocket(acceptSocket *SrtSocket, socket C.SRTSOCKET) (*SrtSocket, error) {
	s := new(SrtSocket)
	s.socket = socket
	s.pktSize = acceptSocket.pktSize
	s.blocking = acceptSocket.blocking
	s.pollTimeout = acceptSocket.pollTimeout

	err := acceptSocket.postconfiguration(s)
	if err != nil {
		return nil, err
	}

	if !s.blocking {
		s.epollIo = C.srt_epoll_create()
		var modes C.int = C.SRT_EPOLL_IN | C.SRT_EPOLL_OUT | C.SRT_EPOLL_ERR
		if C.srt_epoll_add_usock(s.epollIo, s.socket, &modes) == SRT_ERROR {
			return nil, fmt.Errorf("srt epoll: %v", C.GoString(C.srt_getlasterror_str()))
		}
	}

	return s, nil
}

func (s SrtSocket) GetSocket() C.int {
	return s.socket
}

// Listen for incoming connections. The backlog setting defines how many sockets
// may be allowed to wait until they are accepted (excessive connection requests
// are rejected in advance)
func (s SrtSocket) Listen(backlog int) error {
	nbacklog := C.int(backlog)

	sa, salen, err := CreateAddrInet(s.host, s.port)
	if err != nil {
		return err
	}

	res := C.srt_bind(s.socket, sa, C.int(salen))
	if res == SRT_ERROR {
		C.srt_close(s.socket)
		return fmt.Errorf("Error in srt_bind: %v", C.GoString(C.srt_getlasterror_str()))
	}

	res = C.srt_listen(s.socket, nbacklog)
	if res == SRT_ERROR {
		C.srt_close(s.socket)
		return fmt.Errorf("Error in srt_listen: %v", C.GoString(C.srt_getlasterror_str()))
	}

	err = s.postconfiguration(&s)
	if err != nil {
		return fmt.Errorf("Error setting post socket options")
	}

	return nil
}

// Accept an incoming connection
func (s SrtSocket) Accept() (*SrtSocket, *net.UDPAddr, error) {
	if !s.blocking {
		// Socket readiness for connection is checked by polling on WRITE allowed sockets.
		len := C.int(2)
		timeoutMs := C.int64_t(s.pollTimeout)
		ready := [2]C.int{SRT_INVALID_SOCK, SRT_INVALID_SOCK}
		if C.srt_epoll_wait(s.epollConnect, &ready[0], &len, nil, nil, timeoutMs, nil, nil, nil, nil) == -1 {
			return nil, nil, fmt.Errorf("srt accept, epoll error: %s", C.GoString(C.srt_getlasterror_str()))
		}
	}

	var addr syscall.RawSockaddrAny
	sclen := C.int(syscall.SizeofSockaddrAny)
	socket := C.srt_accept(s.socket, (*C.struct_sockaddr)(unsafe.Pointer(&addr)), &sclen)
	if socket == SRT_INVALID_SOCK {
		return nil, nil, fmt.Errorf("srt accept, error accepting the connection: %s", C.GoString(C.srt_getlasterror_str()))
	}

	newSocket, err := newFromSocket(&s, socket)
	if err != nil {
		return nil, nil, fmt.Errorf("new socket could not be created: %w", err)
	}

	udpAddr, err := udpAddrFromSockaddr(&addr)
	if err != nil {
		return nil, nil, err
	}

	return newSocket, udpAddr, nil
}

// Connect to a remote endpoint
func (s SrtSocket) Connect() error {
	sa, salen, err := CreateAddrInet(s.host, s.port)
	if err != nil {
		return err
	}

	runtime.LockOSThread()
	res := C.srt_connect(s.socket, sa, C.int(salen))
	if res == SRT_ERROR {
		C.srt_close(s.socket)
		srt_errno := C.srt_getlasterror(nil)
		runtime.UnlockOSThread()
		switch srt_errno {
		case C.SRT_EINVSOCK:
			return &SrtInvalidSock{}
		case C.SRT_ERDVUNBOUND:
			return &SrtRendezvousUnbound{}
		case C.SRT_ECONNSOCK:
			return &SrtSockConnected{}
		case C.SRT_ECONNREJ:
			return &SrtConnectionRejected{}
		case C.SRT_ENOSERVER:
			return &SrtConnectTimeout{}
		case C.SRT_ESCLOSED:
			return &SrtSocketClosed{}
		}
		return fmt.Errorf("Error in srt_connect")
	}
	runtime.UnlockOSThread()

	if !s.blocking {
		// Socket readiness for connection is checked by polling on WRITE allowed sockets.
		len := C.int(2)
		timeoutMs := C.int64_t(s.pollTimeout)
		ready := [2]C.int{SRT_INVALID_SOCK, SRT_INVALID_SOCK}
		if C.srt_epoll_wait(s.epollConnect, nil, nil, &ready[0], &len, timeoutMs, nil, nil, nil, nil) != -1 {
			state := C.srt_getsockstate(s.socket)
			if state != C.SRTS_CONNECTED {
				return fmt.Errorf("srt connect, connection failed %d", state)
			}
		} else {
			return fmt.Errorf("srt connect, epoll error: %s", C.GoString(C.srt_getlasterror_str()))
		}
	}

	err = s.postconfiguration(&s)
	if err != nil {
		return fmt.Errorf("Error setting post socket options in connect")
	}

	return nil
}

// Read data from the SRT socket
func (s SrtSocket) Read(b []byte) (n int, err error) {
	if !s.blocking {
		len := C.int(2)
		timeoutMs := C.int64_t(s.pollTimeout)
		ready := [2]C.int{SRT_INVALID_SOCK, SRT_INVALID_SOCK}

		if C.srt_epoll_wait(s.epollIo, &ready[0], &len, nil, nil, timeoutMs, nil, nil, nil, nil) == SRT_ERROR {
			if C.srt_getlasterror(nil) == C.SRT_ETIMEOUT {
				return 0, nil
			}
			return 0, fmt.Errorf("error in read:epoll %s", C.GoString(C.srt_getlasterror_str()))
		}
	}

	res := C.srt_recvmsg2(s.socket, (*C.char)(unsafe.Pointer(&b[0])), C.int(len(b)), nil)
	if res == SRT_ERROR {
		return 0, fmt.Errorf("error in read::recv %s", C.GoString(C.srt_getlasterror_str()))
	}

	return int(res), nil
}

// Write data to the SRT socket
func (s SrtSocket) Write(b []byte) (n int, err error) {
	if !s.blocking {
		timeoutMs := C.int64_t(s.pollTimeout)
		len := C.int(2)
		ready := [2]C.int{SRT_INVALID_SOCK, SRT_INVALID_SOCK}
		rlen := C.int(2)
		rready := [2]C.int{SRT_INVALID_SOCK, SRT_INVALID_SOCK}

		if C.srt_epoll_wait(s.epollIo, &rready[0], &rlen, &ready[0], &len, timeoutMs, nil, nil, nil, nil) == SRT_ERROR {
			return 0, fmt.Errorf("error in write:epoll %s", C.GoString(C.srt_getlasterror_str()))
		}
	}

	res := C.srt_sendmsg2(s.socket, (*C.char)(unsafe.Pointer(&b[0])), C.int(len(b)), nil)
	if res == SRT_ERROR {
		return 0, fmt.Errorf("error in write:srt_sendmsg2 %s", C.GoString(C.srt_getlasterror_str()))
	}

	return int(res), nil
}

// Stats - Retrieve stats from the SRT socket
func (s SrtSocket) Stats() (*SrtStats, error) {
	var stats C.SRT_TRACEBSTATS = C.SRT_TRACEBSTATS{}
	var b C.int = 1
	if C.srt_bstats(s.socket, &stats, b) == SRT_ERROR {
		return nil, fmt.Errorf("Error getting stats, %s", C.GoString(C.srt_getlasterror_str()))
	}

	return newSrtStats(&stats), nil
}

// Mode - Return working mode of the SRT socket
func (s SrtSocket) Mode() int {
	return s.mode
}

// PacketSize - Return packet size of the SRT socket
func (s SrtSocket) PacketSize() int {
	return s.pktSize
}

// PollTimeout - Return polling max time, for connect/read/write operations.
// Only applied when socket is in non-blocking mode.
func (s SrtSocket) PollTimeout() time.Duration {
	return time.Duration(s.pollTimeout) * time.Millisecond
}

// SetPollTimeout - Sets polling max time, for connect/read/write operations.
// Only applied when socket is in non-blocking mode.
func (s *SrtSocket) SetPollTimeout(pollTimeout time.Duration) {
	s.pollTimeout = pollTimeout.Milliseconds()
}

// Close the SRT socket
func (s *SrtSocket) Close() {
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
	listenCallbackMutex.Lock()
	if ptr, exists := listenCallbackMap[s.socket]; exists {
		gopointer.Unref(ptr)
	}
	listenCallbackMutex.Unlock()
}

// ListenCallbackFunc specifies a function to be called before a connecting socket is passed to accept
type ListenCallbackFunc func(socket *SrtSocket, version int, addr *net.UDPAddr, streamid string) bool

//export srtListenCBWrapper
func srtListenCBWrapper(arg unsafe.Pointer, socket C.int, hsVersion C.int, peeraddr *C.struct_sockaddr, streamid *C.char) C.int {
	userCB := gopointer.Restore(arg).(ListenCallbackFunc)

	s := new(SrtSocket)
	s.socket = socket
	udpAddr, _ := udpAddrFromSockaddr((*syscall.RawSockaddrAny)(unsafe.Pointer(peeraddr)))

	if userCB(s, int(hsVersion), udpAddr, C.GoString(streamid)) {
		return 0
	}
	return SRT_ERROR
}

// SetListenCallback - set a function to be called early in the handshake before a client
// is handed to accept on a listening socket.
// The connection can be rejected by returning false from the callback.
// See examples/echo-receiver for more details.
func (s SrtSocket) SetListenCallback(cb ListenCallbackFunc) {
	ptr := gopointer.Save(cb)
	C.srt_listen_callback(s.socket, (*C.srt_listen_callback_fn)(C.srtListenCB), ptr)

	listenCallbackMutex.Lock()
	defer listenCallbackMutex.Unlock()
	if listenCallbackMap[s.socket] != nil {
		gopointer.Unref(listenCallbackMap[s.socket])
	}
	listenCallbackMap[s.socket] = ptr
}

// Rejection reasons
var (
	// Start of range for predefined rejection reasons
	RejectionReasonPredefined = int(C.get_srt_error_reject_predefined())

	// General syntax error in the SocketID specification (also a fallback code for undefined cases)
	RejectionReasonBadRequest = RejectionReasonPredefined + 400

	// Authentication failed, provided that the user was correctly identified and access to the required resource would be granted
	RejectionReasonUnauthorized = RejectionReasonPredefined + 401

	// The server is too heavily loaded, or you have exceeded credits for accessing the service and the resource.
	RejectionReasonOverload = RejectionReasonPredefined + 402

	// Access denied to the resource by any kind of reason
	RejectionReasonForbidden = RejectionReasonPredefined + 403

	// Resource not found at this time.
	RejectionReasonNotFound = RejectionReasonPredefined + 404

	// The mode specified in `m` key in StreamID is not supported for this request.
	RejectionReasonBadMode = RejectionReasonPredefined + 405

	// The requested parameters specified in SocketID cannot be satisfied for the requested resource. Also when m=publish and the data format is not acceptable.
	RejectionReasonUnacceptable = RejectionReasonPredefined + 406

	// Start of range for application defined rejection reasons
	RejectionReasonUserDefined = int(C.get_srt_error_reject_predefined())
)

// SetRejectReason - set custom reason for connection reject
func (s SrtSocket) SetRejectReason(value int) error {
	res := C.srt_setrejectreason(s.socket, C.int(value))
	if res == SRT_ERROR {
		return errors.New(C.GoString(C.srt_getlasterror_str()))
	}
	return nil
}

// GetSockOptByte - return byte value obtained with srt_getsockopt
func (s SrtSocket) GetSockOptByte(opt int) (byte, error) {
	var v byte
	l := 1

	err := s.getSockOpt(opt, unsafe.Pointer(&v), &l)
	return v, err
}

// GetSockOptBool - return bool value obtained with srt_getsockopt
func (s SrtSocket) GetSockOptBool(opt int) (bool, error) {
	var v int32
	l := 4

	err := s.getSockOpt(opt, unsafe.Pointer(&v), &l)
	if v == 1 {
		return true, err
	}

	return false, err
}

// GetSockOptInt - return int value obtained with srt_getsockopt
func (s SrtSocket) GetSockOptInt(opt int) (int, error) {
	var v int32
	l := 4

	err := s.getSockOpt(opt, unsafe.Pointer(&v), &l)
	return int(v), err
}

// GetSockOptInt64 - return int64 value obtained with srt_getsockopt
func (s SrtSocket) GetSockOptInt64(opt int) (int64, error) {
	var v int64
	l := 8

	err := s.getSockOpt(opt, unsafe.Pointer(&v), &l)
	return v, err
}

// GetSockOptString - return string value obtained with srt_getsockopt
func (s SrtSocket) GetSockOptString(opt int) (string, error) {
	buf := make([]byte, 256)
	l := len(buf)

	err := s.getSockOpt(opt, unsafe.Pointer(&buf[0]), &l)
	if err != nil {
		return "", err
	}
	return string(buf[:l]), nil
}

// SetSockOptByte - set byte value using srt_setsockopt
func (s SrtSocket) SetSockOptByte(opt int, value byte) error {
	return s.setSockOpt(opt, unsafe.Pointer(&value), 1)
}

// SetSockOptBool - set bool value using srt_setsockopt
func (s SrtSocket) SetSockOptBool(opt int, value bool) error {
	val := int(0)
	if value {
		val = 1
	}
	return s.setSockOpt(opt, unsafe.Pointer(&val), 4)
}

// SetSockOptInt - set int value using srt_setsockopt
func (s SrtSocket) SetSockOptInt(opt int, value int) error {
	return s.setSockOpt(opt, unsafe.Pointer(&value), 4)
}

// SetSockOptInt64 - set int64 value using srt_setsockopt
func (s SrtSocket) SetSockOptInt64(opt int, value int64) error {
	return s.setSockOpt(opt, unsafe.Pointer(&value), 8)
}

// SetSockOptString - set string value using srt_setsockopt
func (s SrtSocket) SetSockOptString(opt int, value string) error {
	return s.setSockOpt(opt, unsafe.Pointer(&[]byte(value)[0]), len(value))
}

func (s SrtSocket) setSockOpt(opt int, data unsafe.Pointer, size int) error {
	res := C.srt_setsockopt(s.socket, 0, C.SRT_SOCKOPT(opt), data, C.int(size))
	if res == -1 {
		return fmt.Errorf("Error calling srt_setsockopt %v", C.GoString(C.srt_getlasterror_str()))
	}

	return nil
}

func (s SrtSocket) getSockOpt(opt int, data unsafe.Pointer, size *int) error {
	res := C.srt_getsockopt(s.socket, 0, C.SRT_SOCKOPT(opt), data, (*C.int)(unsafe.Pointer(size)))
	if res == -1 {
		return fmt.Errorf("Error calling srt_getsockopt %v", C.GoString(C.srt_getlasterror_str()))
	}

	return nil
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
		return ModeFailure, fmt.Errorf("could not set SRTO_RCVSYN flag")
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
			return ModeFailure, fmt.Errorf("could not set LINGER option")
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
		return fmt.Errorf("Error in postconfiguration setting SRTO_SNDSYN")
	}

	res = C.srt_setsockopt(sck.socket, 0, C.SRTO_RCVSYN, unsafe.Pointer(&blocking), C.int(unsafe.Sizeof(blocking)))
	if res == -1 {
		return fmt.Errorf("Error in postconfiguration setting SRTO_RCVSYN")
	}

	err := setSocketOptions(sck.socket, bindingPost, s.options)
	return err
}
