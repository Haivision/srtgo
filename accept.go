package srtgo

/*
#cgo LDFLAGS: -lsrt
#include <srt/srt.h>

SRTSOCKET srt_accept_wrapped(SRTSOCKET lsn, struct sockaddr* addr, int* addrlen, int *srterror, int *syserror)
{
	int ret = srt_accept(lsn, addr, addrlen);
	if (ret < 0) {
		*srterror = srt_getlasterror(syserror);
	}
	return ret;
}

*/
import "C"
import (
	"errors"
	"fmt"
	"net"
	"syscall"
	"unsafe"
)

func srtAcceptImpl(lsn C.SRTSOCKET, addr *C.struct_sockaddr, addrlen *C.int) (C.SRTSOCKET, error) {
	srterr := C.int(0)
	syserr := C.int(0)
	socket := C.srt_accept_wrapped(lsn, addr, addrlen, &srterr, &syserr)
	if srterr != 0 {
		srterror := SRTErrno(srterr)
		if syserr < 0 {
			srterror.wrapSysErr(syscall.Errno(syserr))
		}
		return socket, srterror
	}
	return socket, nil
}

// Accept an incoming connection
//
// The accept path deliberately mirrors Read and Write: try first, and only
// wait when SRT reports there is nothing pending. Waiting first is what made
// a listener lose connections. Sockets are registered edge-triggered, and
// libsrt collapses every connection that arrives between two srt_epoll_uwait
// calls into a single SRT_EPOLL_IN notice, which it then clears once reported.
// A backlog therefore produces exactly one readiness edge no matter how many
// connections it holds, and libsrt raises no further edge while the readiness
// bit is still set -- so a listener that accepted one connection and went back
// to waiting would never be woken for the rest, nor for any later arrival.
// Attempting the accept up front drains the backlog across successive calls
// without needing an edge that is never coming.
func (s SrtSocket) Accept() (*SrtSocket, *net.UDPAddr, error) {
	var addr syscall.RawSockaddrAny
	sclen := C.int(sizeofSockaddrAny)

	//Fastpath
	if !s.blocking {
		s.pd.reset(ModeRead)
	}
	socket, err := srtAcceptImpl(s.socket, (*C.struct_sockaddr)(unsafe.Pointer(&addr)), &sclen)
	for {
		if !errors.Is(err, error(EAsyncRCV)) || s.blocking {
			break
		}
		if err = s.pd.wait(ModeRead); err != nil {
			return nil, nil, err
		}
		sclen = C.int(sizeofSockaddrAny)
		socket, err = srtAcceptImpl(s.socket, (*C.struct_sockaddr)(unsafe.Pointer(&addr)), &sclen)
	}
	if err != nil {
		return nil, nil, err
	}
	if socket == SRT_INVALID_SOCK {
		return nil, nil, fmt.Errorf("srt accept, error accepting the connection: %w", srtGetAndClearError())
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
