package srtgo

/*
#cgo LDFLAGS: -lsrt
#include <srt/srt.h>
*/
import "C"
import (
	"runtime"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

const (
	pollDefault = int32(iota)
	pollReady   = int32(iota)
	pollWait    = int32(iota)
)

type pollDesc struct {
	lock      sync.Mutex
	closing   bool
	fd        C.SRTSOCKET
	pollErr   bool
	unblockRd chan interface{}
	rdState   int32
	rdLock    sync.Mutex
	rd        int64
	rdSeq     int64
	rt        *time.Timer
	unblockWr chan interface{}
	wrState   int32
	wrLock    sync.Mutex
	wd        int64
	wdSeq     int64
	wt        *time.Timer
	pollS     *pollServer
}

func (pd *pollDesc) Wait(mode int) error {
	if err := pd.checkPollErr(mode); err != nil {
		return err
	}
	state := &pd.rdState
	unblockChan := pd.unblockRd
	if mode == 'r' {
		pd.rdLock.Lock()
		defer pd.rdLock.Unlock()
	} else if mode == 'w' {
		state = &pd.wrState
		unblockChan = pd.unblockWr
		pd.wrLock.Lock()
		defer pd.wrLock.Unlock()
	}

	for {
		old := *state
		if old == pollReady {
			*state = pollDefault
			return nil
		}
		if atomic.CompareAndSwapInt32(state, pollDefault, pollWait) {
			break
		}
	}
	<-unblockChan
	err := pd.checkPollErr(mode)
	pd.reset(mode)
	return err
}

func (pd *pollDesc) Close() {
	pd.lock.Lock()
	defer pd.lock.Unlock()
	if pd.closing {
		return
	}
	pd.closing = true
	close(pd.unblockRd)
	close(pd.unblockWr)
	pd.pollS.pollClose(pd)
}

func (pd *pollDesc) checkPollErr(mode int) error {
	pd.lock.Lock()
	defer pd.lock.Unlock()
	if pd.closing {
		return &SrtSocketClosed{}
	}

	if mode == 'r' && pd.rd < 0 || mode == 'w' && pd.wd < 0 {
		return &SrtEpollTimeout{}
	}

	if pd.pollErr {
		pd.pollErr = false //Consume the error
		return &SrtSocketClosed{}
	}

	return nil
}

func (pd *pollDesc) SetDeadline(d time.Time) {
	pd.setDeadline(d, 'r'+'w')
}

func (pd *pollDesc) SetReadDeadline(d time.Time) {
	pd.setDeadline(d, 'r')
}

func (pd *pollDesc) SetWriteDeadline(d time.Time) {
	pd.setDeadline(d, 'w')
}

func (pd *pollDesc) deadlinefunc(seq int64, mode int) func() {
	return func() {
		if mode == 'r' {
			if seq == pd.rdSeq {
				pd.unblock('r', false, false)
			}
		}
		if mode == 'w' {
			if seq == pd.wdSeq {
				pd.unblock('w', false, false)
			}
		}
	}
}

func (pd *pollDesc) setDeadline(t time.Time, mode int) {
	pd.lock.Lock()
	defer pd.lock.Unlock()
	var d int64
	if !t.IsZero() {
		d = int64(time.Until(t))
		if d == 0 {
			d = -1
		}
	}
	if mode == 'r' || mode == 'r'+'w' {
		if pd.rd > 0 {
			pd.rt.Stop()
			pd.rt = nil
		}
		pd.rd = d
		pd.rdSeq++
		if d > 0 {
			pd.rt = time.AfterFunc(time.Duration(d), pd.deadlinefunc(pd.rdSeq, 'r'))
		}
		if d < 0 {
			pd.unblock('r', false, false)
		}
	}
	if mode == 'w' || mode == 'r'+'w' {
		if pd.wd > 0 {
			pd.wt.Stop()
			pd.wt = nil
		}
		pd.wd = d
		pd.wdSeq++
		if d > 0 {
			pd.wt = time.AfterFunc(time.Duration(d), pd.deadlinefunc(pd.wdSeq, 'w'))
		}
		if d < 0 {
			pd.unblock('w', false, false)
		}
	}
	if d == 0 {
		return
	}
}

func (pd *pollDesc) unblock(mode int, pollerr, ioready bool) {
	if pollerr {
		pd.lock.Lock()
		pd.pollErr = pollerr
		pd.lock.Unlock()
	}
	state := &pd.rdState
	unblockChan := pd.unblockRd
	if mode == 'w' {
		state = &pd.wrState
		unblockChan = pd.unblockWr
	}
	old := atomic.LoadInt32(state)
	if ioready {
		atomic.StoreInt32(state, pollReady)
	}
	if old == pollWait {
		unblockChan <- struct{}{}
	}
}

func (pd *pollDesc) reset(mode int) {
	pd.lock.Lock()
	defer pd.lock.Unlock()
	if mode == 'r' {
		pd.rdState = pollDefault
	} else if mode == 'w' {
		pd.wrState = pollDefault
	}
}

func PollDescInit(s C.SRTSOCKET) *pollDesc {
	pd := new(pollDesc)
	pd.lock.Lock()
	defer pd.lock.Unlock()
	pd.fd = s
	pd.rdState = pollDefault
	pd.wrState = pollDefault
	pd.pollS = pollServerH
	pd.unblockRd = make(chan interface{})
	pd.unblockWr = make(chan interface{})
	pd.rdSeq++
	pd.wdSeq++
	pd.pollS.pollOpen(pd)
	runtime.SetFinalizer(pd, func(obj interface{}) {
		pd := obj.(*pollDesc)
		pd.Close()
	})
	return pd
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
	ret := C.srt_epoll_remove_usock(p.srtEpollDescr, pd.fd)
	if ret == -1 {
		panic("ERROR REMOVING FD FROM EPOLL")
	}
	p.pollDescLock.Lock()
	delete(p.pollDescs, pd.fd)
	p.pollDescLock.Unlock()
}

var (
	pollServerH *pollServer
)

func init() {
	pollServerH = &pollServer{
		srtEpollDescr: C.srt_epoll_create(),
		pollDescs:     make(map[C.SRTSOCKET]*pollDesc),
	}
	go pollServerH.run()
}

func (p *pollServer) addCanary() {
	p.canaryLock.Lock()
	defer p.canaryLock.Unlock()
	p.canary = C.srt_create_socket()
	events := C.int(C.SRT_EPOLL_ERR)
	C.srt_epoll_add_usock(pollServerH.srtEpollDescr, pollServerH.canary, &events)
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
