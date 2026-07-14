package srtgo

/*
#cgo LDFLAGS: -lsrt
#include <srt/srt.h>

int srt_recvmsg2_wrapped(SRTSOCKET u, char* buf, int len, SRT_MSGCTRL *mctrl, int *srterror, int *syserror)
{
	int ret = srt_recvmsg2(u, buf, len, mctrl);
	if (ret < 0) {
		*srterror = srt_getlasterror(syserror);
	}
	return ret;
}

*/
import "C"
import (
	"errors"
	"syscall"
	"unsafe"
)

func srtRecvMsg2Impl(u C.SRTSOCKET, buf []byte, msgctrl *C.SRT_MSGCTRL) (n int, err error) {
	srterr := C.int(0)
	syserr := C.int(0)
	n = int(C.srt_recvmsg2_wrapped(u, (*C.char)(unsafe.Pointer(&buf[0])), C.int(len(buf)), msgctrl, &srterr, &syserr))
	if n < 0 {
		srterror := SRTErrno(srterr)
		if syserr < 0 {
			srterror.wrapSysErr(syscall.Errno(syserr))
		}
		err = srterror
		n = 0
	}
	return
}

// Read data from the SRT socket
func (s SrtSocket) Read(b []byte) (n int, err error) {
	// Fast path: try reading immediately
	n, err = srtRecvMsg2Impl(s.socket, b, nil)

	// If successful or blocking mode, return immediately
	if err == nil || s.blocking || !errors.Is(err, error(EAsyncRCV)) {
		return
	}

	// Non-blocking mode: wait for data to be available
	if !s.blocking {
		s.pd.reset(ModeRead)
		if waitErr := s.pd.wait(ModeRead); waitErr != nil {
			return 0, waitErr
		}
		// Try reading again after waiting
		n, err = srtRecvMsg2Impl(s.socket, b, nil)
	}

	return
}
