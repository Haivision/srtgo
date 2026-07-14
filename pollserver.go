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
	eid := C.srt_epoll_create()
	C.srt_epoll_set(eid, C.SRT_EPOLL_ENABLE_EMPTY)
	phctx = &pollServer{
		srtEpollDescr: eid,
		pollDescs:     make(map[C.SRTSOCKET]*pollDesc),
	}
	go phctx.run()
}

type pollServer struct {
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
	//Broken/closed sockets get removed internally by SRT lib
	if sockstate == C.SRTS_BROKEN || sockstate == C.SRTS_CLOSING || sockstate == C.SRTS_CLOSED || sockstate == C.SRTS_NONEXIST {
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

func (p *pollServer) run() {
	// Use a reasonable timeout instead of infinite to prevent busy waiting
	// and allow for graceful shutdown
	timeoutMs := C.int64_t(100) // 100ms timeout
	fds := [128]C.SRT_EPOLL_EVENT{}
	fdlen := C.int(128)

	for {
		res := C.srt_epoll_uwait(p.srtEpollDescr, &fds[0], fdlen, timeoutMs)
		if res == 0 {
			// Timeout occurred, this is normal with finite timeout
			continue
		} else if res == -1 {
			// Check if this is a recoverable error
			errno := C.srt_getlasterror(nil)
			if errno == C.SRT_ETIMEOUT {
				continue // Timeout is expected, continue polling
			}
			panic("srt_epoll_error")
		} else if res > 0 {
			max := int(res)
			if fdlen < res {
				max = int(fdlen)
			}

			// Process events in batches to reduce lock contention
			p.processEvents(fds[:max])
		}
	}
}

// processEvents handles a batch of events with optimized locking
func (p *pollServer) processEvents(events []C.SRT_EPOLL_EVENT) {
	// Take a snapshot of poll descriptors to minimize lock time
	p.pollDescLock.Lock()
	eventPds := make([]*pollDesc, len(events))
	eventTypes := make([]C.int, len(events))

	for i, event := range events {
		if pd, exists := p.pollDescs[event.fd]; exists {
			eventPds[i] = pd
			eventTypes[i] = C.int(event.events)
		}
	}
	p.pollDescLock.Unlock()

	// Process events without holding the main lock
	for i, pd := range eventPds {
		if pd == nil {
			continue
		}

		eventFlags := eventTypes[i]
		if eventFlags&C.SRT_EPOLL_ERR != 0 {
			pd.unblock(ModeRead, true, false)
			pd.unblock(ModeWrite, true, false)
			continue
		}
		if eventFlags&C.SRT_EPOLL_IN != 0 {
			pd.unblock(ModeRead, false, true)
		}
		if eventFlags&C.SRT_EPOLL_OUT != 0 {
			pd.unblock(ModeWrite, false, true)
		}
	}
}
