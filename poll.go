package srtgo

/*
#cgo LDFLAGS: -lsrt
#include <srt/srt.h>
*/
import "C"
import (
	"fmt"
	"runtime"
	"sync"
	"time"
	"unsafe"
)

type pollState int32

const (
	pollDefault = pollState(iota)
	pollReady   = pollState(iota)
	pollWait    = pollState(iota)
)

type pollDesc struct {
	lock      sync.Mutex
	fd        C.SRTSOCKET
	pollErr   bool
	unblockRd chan interface{}
	rdState   pollState
	rdLock    sync.Mutex
	rd        int64
	rdSeq     int64
	unblockWr chan interface{}
	wrState   pollState
	wrLock    sync.Mutex
	wd        int64
	wdSeq     int64
	pollS     *pollServer
}

var pollDescPool = sync.Pool{
	New: func() interface{} {
		return &pollDesc{}
	},
}

func (pd *pollDesc) Wait(mode int) error {
	if err := pd.checkPollErr(mode); err != nil {
		return err
	}
	if mode == 'r' {
		pd.rdLock.Lock()
		defer pd.rdLock.Unlock()
		/*
		if pd.rdState == pollReady {
			pd.reset(mode)
			return nil
		}*/
		pd.reset(mode)
		pd.lock.Lock()
		pd.rdState = pollWait
		pd.lock.Unlock()
		fmt.Printf("WAIT READ\n\n\n\n")
		<-pd.unblockRd
		fmt.Printf("DONE WAIT READ\n\n\n\n")
		err := pd.checkPollErr(mode)
		pd.reset(mode)
		return err
	}
	pd.wrLock.Lock()
	defer pd.wrLock.Unlock()
	/*
	if pd.wrState == pollReady {
		pd.reset(mode)
		return nil
	}*/
	pd.reset(mode)
	pd.lock.Lock()
	pd.wrState = pollWait
	pd.lock.Unlock()
	fmt.Printf("WAIT WRITE\n\n\n\n")
	<-pd.unblockWr
	fmt.Printf("DONE WAIT WRITE\n\n\n\n")
	err := pd.checkPollErr(mode)
	pd.reset(mode)
	return err
}

func (pd *pollDesc) Close() {
	pd.lock.Lock()
	defer pd.lock.Unlock()
	close(pd.unblockRd)
	close(pd.unblockWr)
	pd.pollS.pollClose(pd)
	//TODO: figure out a way to cleanly return these without causing any potential null pointer migrations
	//pollDescPool.Put(pd)

}

func (pd *pollDesc) checkPollErr(mode int) error {
	if mode == 'r' && pd.rd < 0 || mode == 'w' && pd.wd < 0 {
		return &SrtEpollTimeout{}
	}
	if pd.pollErr {
		return &SrtSocketClosed{}
	}
	return nil
}

func (pd *pollDesc) SetDeadline(d time.Time) {
	pd.setDeadline(d, true, true)
}

func (pd *pollDesc) SetReadDeadline(d time.Time) {
	pd.setDeadline(d, true, false)
}

func (pd *pollDesc) SetWriteDeadline(d time.Time) {
	pd.setDeadline(d, false, true)
}

func (pd *pollDesc) setDeadline(t time.Time, read, write bool) {
	pd.lock.Lock()
	defer pd.lock.Unlock()
	var d int64
	if !t.IsZero() {
		d = int64(time.Until(t))
		if d == 0 {
			d = -1
		}
	}
	if read {
		pd.rd = d
		pd.rdSeq++
	}
	if write {
		pd.wd = d
		pd.wdSeq++
	}
	if d == 0 {
		return
	}
	go func(r, w bool, rseq, wseq int64, pd *pollDesc) {
		<-time.After(time.Duration(d) * time.Nanosecond)
		pd.lock.Lock()
		if r && rseq == pd.rdSeq {
			pd.rd = -1
			pd.unblock('r', false, false)
		}
		if w && wseq == pd.wdSeq {
			pd.wd = -1
			pd.unblock('w', false, false)
		}
		pd.lock.Unlock()
	}(read, write, pd.rdSeq, pd.wdSeq, pd)
}

func (pd *pollDesc) unblock(mode int, pollerr, ioready bool) {
	pd.pollErr = pollerr
	if mode == 'r' {
		waiting := pd.rdState == pollWait
		pd.rdState = 0
		if ioready {
			pd.rdState = pollReady
		}
		if waiting {
			pd.unblockRd <- struct{}{}
		}
		return
	}
	waiting := pd.wrState == pollWait
	pd.wrState = 0
	if ioready {
		pd.wrState = pollReady
	}
	if waiting {
		pd.unblockWr <- struct{}{}
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

func PollDescInit(s *SrtSocket) *pollDesc {
	pd := pollDescPool.Get().(*pollDesc)
	pd.lock.Lock()
	defer pd.lock.Unlock()
	pd.fd = s.socket
	pd.rdState = pollDefault
	pd.wrState = pollDefault
	pd.pollS = pollServerH
	pd.unblockRd = make(chan interface{})
	pd.unblockWr = make(chan interface{})
	pd.rdSeq++
	pd.wdSeq++
	pd.pollS.pollOpen(pd)
	runtime.SetFinalizer(s, func(obj interface{}) {
		s := obj.(*SrtSocket)
		s.pd.Close()
		s.pd = nil
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
	fmt.Printf("Adding %d to epoll\n\n\n", pd.fd)
	ret := C.srt_epoll_add_usock(p.srtEpollDescr, pd.fd, (*C.int)(unsafe.Pointer(&events)))
	if ret == -1 {
		panic("ERROR ADDING FD TO EPOLL")
	}
	p.pollDescLock.Lock()
	p.pollDescs[pd.fd] = pd
	p.pollDescLock.Unlock()
}

func (p *pollServer) pollClose(pd *pollDesc) {
	fmt.Printf("Removing %d from epoll\n\n\n", pd.fd)
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
	p.addCanary()
	timeoutMs := C.int64_t(-1)
	fds := [128]C.SRT_EPOLL_EVENT{}
	len := C.int(128)
	for {
		res := C.srt_epoll_uwait(p.srtEpollDescr, &fds[0], len, timeoutMs)
		if res == 0 {
			continue //Shouldn't happen with -1
		} else if res == -1 {
			panic("srt_epoll_error")
		} else if res > 0 {
			max := int(res)
			if len < res {
				max = int(len)
			}
			p.pollDescLock.Lock()
			for i := 0; i < max; i++ {
				s := fds[i].fd
				events := fds[i].fd
				if s == p.canary && (events&C.SRT_EPOLL_ERR) > 0 {
					p.addCanary()
					continue
				}
				pd := p.pollDescs[s]
				pd.lock.Lock()
				if events&C.SRT_EPOLL_ERR != 0 {
					sockstate := C.srt_getsockstate(pd.fd)
					fmt.Printf("EPOLL_ERR: %d sockstate: %d\n\n\n", pd.fd, sockstate)
					switch sockstate {
					case C.SRTS_BROKEN, C.SRTS_CLOSING, C.SRTS_CLOSED, C.SRTS_NONEXIST:
						pd.unblock('r', true, false)
						pd.unblock('w', true, false)
					default:
						//
					}
					pd.lock.Unlock()
					continue
				}
				if events&C.SRT_EPOLL_IN != 0 {
					fmt.Printf("SRT POLL IN!\n\n\n")
					pd.unblock('r', false, true)
				}
				if events&C.SRT_EPOLL_OUT != 0 {
					fmt.Printf("SRT POLL OUT!\n\n\n")
					pd.unblock('w', false, true)
				}
				pd.lock.Unlock()
			}
			p.pollDescLock.Unlock()
		}
	}
}
