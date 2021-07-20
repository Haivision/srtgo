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
	}
	return
}

// Read data from the SRT socket
func (s SrtSocket) Read(b []byte) (n int, err error) {
	//Fastpath
	n, err = srtRecvMsg2Impl(s.socket, b, nil)

	if err != nil {
		if errors.Is(err, error(EAsyncRCV)) && !s.blocking {
			s.pd.wait('r')
			n, err = srtRecvMsg2Impl(s.socket, b, nil)
		}
	}
	return
}
