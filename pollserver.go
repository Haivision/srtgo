package srtgo

/*
#cgo LDFLAGS: -lsrt
#include <srt/srt.h>
*/
import "C"

import (
	"sync"
	"unsafe"
)

var (
	phctx *pollServer
	once  sync.Once
)

func pollServerCtx() *pollServer {
	once.Do(pollServerCtxInit)
	return phctx
}

func pollServerCtxInit() {
	phctx = &pollServer{
		srtEpollDescr: C.srt_epoll_create(),
		pollDescs:     make(map[C.SRTSOCKET]*pollDesc),
	}
	go phctx.run()
}

type pollServer struct {
	canary        C.SRTSOCKET
	canaryLock    sync.Mutex
	srtEpollDescr C.int
	pollDescLock  sync.Mutex
	pollDescs     map[C.SRTSOCKET]*pollDesc
}

func (p *pollServer) pollOpen(pd *pollDesc) {
	//use uint because otherwise with ET it would overflow :/ (srt should accept an uint instead, or fix it's SRT_EPOLL_ET definition)
	events := C.uint(C.SRT_EPOLL_IN | C.SRT_EPOLL_OUT | C.SRT_EPOLL_ERR | C.SRT_EPOLL_ET)
	//via unsafe.Pointer because we cannot cast *C.uint to *C.int directly
	//block poller
	p.pollDescLock.Lock()
	ret := C.srt_epoll_add_usock(p.srtEpollDescr, pd.fd, (*C.int)(unsafe.Pointer(&events)))
	if ret == -1 {
		panic("ERROR ADDING FD TO EPOLL")
	}
	p.pollDescs[pd.fd] = pd
	p.pollDescLock.Unlock()
}

func (p *pollServer) pollClose(pd *pollDesc) {
	sockstate := C.srt_getsockstate(pd.fd)
	//Broken sockets get removed internally by SRT lib
	if sockstate == C.SRTS_BROKEN {
		return
	}
	ret := C.srt_epoll_remove_usock(p.srtEpollDescr, pd.fd)
	if ret == -1 {
		panic("ERROR REMOVING FD FROM EPOLL")
	}
	p.pollDescLock.Lock()
	delete(p.pollDescs, pd.fd)
	p.pollDescLock.Unlock()
}

func init() {

}

func (p *pollServer) addCanary() {
	p.canaryLock.Lock()
	defer p.canaryLock.Unlock()
	p.canary = C.srt_create_socket()
	events := C.int(C.SRT_EPOLL_ERR)
	C.srt_epoll_add_usock(p.srtEpollDescr, p.canary, &events)
}

func (p *pollServer) run() {
	p.addCanary() //prevents epoll error due to not having sockets
	timeoutMs := C.int64_t(-1)
	fds := [128]C.SRT_EPOLL_EVENT{}
	fdlen := C.int(128)
	for {
		res := C.srt_epoll_uwait(p.srtEpollDescr, &fds[0], fdlen, timeoutMs)
		if res == 0 {
			continue //Shouldn't happen with -1
		} else if res == -1 {
			panic("srt_epoll_error")
		} else if res > 0 {
			max := int(res)
			if fdlen < res {
				max = int(fdlen)
			}
			p.pollDescLock.Lock()
			for i := 0; i < max; i++ {
				s := fds[i].fd
				events := fds[i].events

				if s == p.canary && (events&C.SRT_EPOLL_ERR) > 0 {
					p.addCanary()
					continue
				}
				pd := p.pollDescs[s]
				if events&C.SRT_EPOLL_ERR != 0 {
					pd.unblock('r', true, false)
					pd.unblock('w', true, false)
					continue
				}
				if events&C.SRT_EPOLL_IN != 0 {
					pd.unblock('r', false, true)
				}
				if events&C.SRT_EPOLL_OUT != 0 {
					pd.unblock('w', false, true)
				}
			}
			p.pollDescLock.Unlock()
		}
	}
}
