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
	//phctx is nil whenever no poll server is running: before the first socket
	//is created, and again after CleanupSRT. It is deliberately not a
	//sync.Once -- a shut down poll server must be replaceable, so that
	//CleanupSRT followed by InitSRT works the way it did before shutdown
	//existed.
	phctxLock sync.Mutex
	phctx     *pollServer
)

func pollServerCtx() *pollServer {
	phctxLock.Lock()
	defer phctxLock.Unlock()
	if phctx == nil {
		phctx = newPollServer()
	}
	return phctx
}

func newPollServer() *pollServer {
	eid := C.srt_epoll_create()
	C.srt_epoll_set(eid, C.SRT_EPOLL_ENABLE_EMPTY)
	p := &pollServer{
		srtEpollDescr: eid,
		pollDescs:     make(map[C.SRTSOCKET]*pollDesc),
		stop:          make(chan struct{}),
		done:          make(chan struct{}),
	}
	go p.run()
	return p
}

type pollServer struct {
	srtEpollDescr C.int
	pollDescLock  sync.Mutex
	pollDescs     map[C.SRTSOCKET]*pollDesc
	stop          chan struct{}
	done          chan struct{}
	stopOnce      sync.Once
}

// shutdown stops the poll loop and releases the epoll, in that order.
//
// The ordering is the whole point. run() spends nearly all its time parked
// inside srt_epoll_uwait, and tearing SRT down underneath a thread that is
// still in there is how the library ends up faulting during process exit.
// shutdown signals the loop, waits for it to actually leave C, and only then
// releases the epoll. It is idempotent and safe to call when no poll server
// was ever started.
func (p *pollServer) shutdown() {
	p.stopOnce.Do(func() {
		close(p.stop)
		<-p.done
		C.srt_epoll_release(p.srtEpollDescr)
	})
}

// pollServerShutdown stops the process-wide poll server if one is running, and
// clears it so a later socket starts a fresh one rather than reusing an epoll
// that has already been released.
func pollServerShutdown() {
	phctxLock.Lock()
	p := phctx
	phctx = nil
	phctxLock.Unlock()
	if p == nil {
		return
	}
	p.shutdown()
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

	//Remove from the map, so closed sockets don't slowly fill the map.
	p.pollDescLock.Lock()
	delete(p.pollDescs, pd.fd)
	p.pollDescLock.Unlock()

	//Broken/closed sockets get removed internally by SRT lib
	if sockstate == C.SRTS_BROKEN || sockstate == C.SRTS_CLOSING || sockstate == C.SRTS_CLOSED || sockstate == C.SRTS_NONEXIST {
		return
	}
	ret := C.srt_epoll_remove_usock(p.srtEpollDescr, pd.fd)
	if ret == -1 {
		panic("ERROR REMOVING FD FROM EPOLL")
	}
}

func init() {

}

func (p *pollServer) run() {
	defer close(p.done)
	//A finite timeout is what makes shutdown possible at all: with an infinite
	//wait this goroutine would sit inside C indefinitely and could never
	//observe p.stop. The wakeups are cheap and only happen while idle.
	timeoutMs := C.int64_t(100)
	fds := [128]C.SRT_EPOLL_EVENT{}
	fdlen := C.int(128)
	for {
		select {
		case <-p.stop:
			return
		default:
		}
		res := C.srt_epoll_uwait(p.srtEpollDescr, &fds[0], fdlen, timeoutMs)
		if res == 0 {
			continue //timeout expired with nothing ready
		} else if res == -1 {
			//A failing uwait during shutdown is expected, not a bug.
			select {
			case <-p.stop:
				return
			default:
			}
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

				pd := p.pollDescs[s]
				if pd == nil {
					continue
				}
				if events&C.SRT_EPOLL_ERR != 0 {
					pd.unblock(ModeRead, true, false)
					pd.unblock(ModeWrite, true, false)
					continue
				}
				if events&C.SRT_EPOLL_IN != 0 {
					pd.unblock(ModeRead, false, true)
				}
				if events&C.SRT_EPOLL_OUT != 0 {
					pd.unblock(ModeWrite, false, true)
				}
			}
			p.pollDescLock.Unlock()
		}
	}
}
