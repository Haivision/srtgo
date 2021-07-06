package srtgo

// #cgo LDFLAGS: -lsrt
// #include <srt/srt.h>
import "C"

type SrtMsgCtrl struct {
	MsgTTL   int   // TTL for a message (millisec), default -1 (no TTL limitation)
	InOrder  int   // Whether a message is allowed to supersede partially lost one. Unused in stream and live mode.
	Boundary int   // 0:mid pkt, 1(01b):end of frame, 2(11b):complete frame, 3(10b): start of frame
	SrcTime  int64 // source time since epoch (usec), 0: use internal time (sender)
	PktSeq   int32 // sequence number of the first packet in received message (unused for sending)
	MsgNo    int32 // message number (output value for both sending and receiving)
}

func newSrtMsgCtrl(ctrl *C.SRT_MSGCTRL) (res SrtMsgCtrl) {
	res.MsgTTL = int(ctrl.msgttl)
	res.InOrder = int(ctrl.inorder)
	res.Boundary = int(ctrl.boundary)
	res.SrcTime = int64(ctrl.srctime)
	res.PktSeq = int32(ctrl.pktseq)
	res.MsgNo = int32(ctrl.msgno)
	return
}
