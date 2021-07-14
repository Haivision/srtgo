package srtgo

/*
#cgo LDFLAGS: -lsrt
#include <srt/srt.h>

int srt_sendmsg2_wrapped(SRTSOCKET u, const char* buf, int len, SRT_MSGCTRL *mctrl, int *srterror, int *syserror)
{
	int ret = srt_sendmsg2(u, buf, len, mctrl);
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
	"runtime"
	"syscall"
	"unsafe"
)

func srtSendMsg2Impl(u C.SRTSOCKET, buf []byte, msgctrl *C.SRT_MSGCTRL) (n int, err error) {
	srterr := C.int(0)
	syserr := C.int(0)
	n = int(C.srt_sendmsg2_wrapped(u, (*C.char)(unsafe.Pointer(&buf[0])), C.int(len(buf)), msgctrl, &srterr, &syserr))
	if n < 0 {
		srterror := SRTErrno(srterr)
		if syserr < 0 {
			srterror.wrapSysErr(syscall.Errno(syserr))
		}
		err = srterror
	}
	return
}

// Write data to the SRT socket
func (s SrtSocket) Write(b []byte) (n int, err error) {

	//Fastpath:
	n, err = srtSendMsg2Impl(s.socket, b, nil)

	if err != nil {
		if errors.Is(err, error(EAsyncSND)) && !s.blocking {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			timeoutMs := C.int64_t(s.pollTimeout)
			fds := [1]C.SRT_EPOLL_EVENT{}
			len := C.int(1)
			res := C.srt_epoll_uwait(s.epollOut, &fds[0], len, timeoutMs)
			if res == 0 {
				return 0, &SrtEpollTimeout{}
			}
			if res == SRT_ERROR {
				return 0, fmt.Errorf("error in write:epoll %w", srtGetAndClearError())
			}
			if fds[0].events&C.SRT_EPOLL_ERR > 0 {
				return 0, &SrtSocketClosed{}
			}
			n, err = srtSendMsg2Impl(s.socket, b, nil)
		}
	}
	return
}
