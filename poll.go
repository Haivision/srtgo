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

func pollDescInit(s C.SRTSOCKET) *pollDesc {
	pd := new(pollDesc)
	pd.lock.Lock()
	defer pd.lock.Unlock()
	pd.fd = s
	pd.rdState = pollDefault
	pd.wrState = pollDefault
	pd.pollS = pollServerCtx()
	pd.unblockRd = make(chan interface{})
	pd.unblockWr = make(chan interface{})
	pd.rdSeq++
	pd.wdSeq++
	pd.pollS.pollOpen(pd)
	runtime.SetFinalizer(pd, func(obj interface{}) {
		pd := obj.(*pollDesc)
		pd.close()
	})
	return pd
}

func (pd *pollDesc) wait(mode int) error {
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

func (pd *pollDesc) close() {
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
		return &SrtSocketClosed{}
	}

	return nil
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
