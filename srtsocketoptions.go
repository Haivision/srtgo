package srtgo

// #cgo LDFLAGS: -lsrt
// #include <srt/srt.h>
import "C"

import (
    "syscall"
    "strconv"
    "unsafe"
    "log"
)

const (
    TransTypeLive = 0
    TransTypeFile = 1
)

const (
	TInteger   = 0
	TInteger64 = 1
	TString    = 2
    TBoolean   = 3
    TTransType = 4
)

type socketOption struct {
	name       string
	level      int
	option	   int
	binding    int
	dataType   int
}

// List of possible socket options
var SocketOptions = []socketOption{
	{ "transtype", 0, C.SRTO_TRANSTYPE, BindingPre, TTransType },
	{ "maxbw", 0, C.SRTO_MAXBW, BindingPre, TInteger64 },
    { "pbkeylen", 0, C.SRTO_PBKEYLEN, BindingPre, TInteger },
    { "passphrase", 0, C.SRTO_PASSPHRASE, BindingPre, TString },
    { "mss", 0, C.SRTO_MSS, BindingPre, TInteger },
    { "fc", 0, C.SRTO_FC, BindingPre, TInteger },
    { "sndbuf", 0, C.SRTO_SNDBUF, BindingPre, TInteger },
    { "rcvbuf", 0, C.SRTO_RCVBUF, BindingPre, TInteger },
    { "ipttl", 0, C.SRTO_IPTTL, BindingPre, TInteger },
    { "iptos", 0, C.SRTO_IPTOS, BindingPre, TInteger },
    { "inputbw", 0, C.SRTO_INPUTBW, BindingPost, TInteger64 },
    { "oheadbw", 0, C.SRTO_OHEADBW, BindingPost, TInteger },
    { "latency", 0, C.SRTO_LATENCY, BindingPre, TInteger },
    { "tsbpdmode", 0, C.SRTO_TSBPDMODE, BindingPre, TBoolean },
    { "tlpktdrop", 0, C.SRTO_TLPKTDROP, BindingPre, TBoolean },
    { "snddropdelay", 0, C.SRTO_SNDDROPDELAY, BindingPost, TInteger },
    { "nakreport", 0, C.SRTO_NAKREPORT, BindingPre, TBoolean },
    { "conntimeo", 0, C.SRTO_CONNTIMEO, BindingPre, TInteger },
    { "lossmaxttl", 0, C.SRTO_LOSSMAXTTL, BindingPre, TInteger },
    { "rcvlatency", 0, C.SRTO_RCVLATENCY, BindingPre, TInteger },
    { "peerlatency", 0, C.SRTO_PEERLATENCY, BindingPre, TInteger },
    { "minversion", 0, C.SRTO_MINVERSION, BindingPre, TInteger },
    { "streamid", 0, C.SRTO_STREAMID, BindingPre, TString },
    { "congestion", 0, C.SRTO_CONGESTION, BindingPre, TString },
    { "messageapi", 0, C.SRTO_MESSAGEAPI, BindingPre, TBoolean },
    { "payloadsize", 0, C.SRTO_PAYLOADSIZE, BindingPre, TInteger },
    { "kmrefreshrate", 0, C.SRTO_KMREFRESHRATE, BindingPre, TInteger },
    { "kmpreannounce", 0, C.SRTO_KMPREANNOUNCE, BindingPre, TInteger },
    { "enforcedencryption", 0, C.SRTO_ENFORCEDENCRYPTION, BindingPre, TBoolean },
    { "peeridletimeo", 0, C.SRTO_PEERIDLETIMEO, BindingPre, TInteger },
    { "packetfilter", 0, C.SRTO_PACKETFILTER, BindingPre, TString },
}

func SetSocketLingerOption(s C.int, li int32) error {
	var lin syscall.Linger;
	lin.Linger = li;
	if (lin.Linger > 0 ) {
		lin.Onoff = 1
	} else {
		lin.Onoff = 0
	}
	// C.srt_setsockopt(s.socket, BindingPre, C.SRTO_LINGER, unsafe.Pointer(&lin), C.int(unsafe.Sizeof(lin)))
	return syscall.SetsockoptLinger(int(s), BindingPre, syscall.SO_LINGER, &lin)
}

// Set socket options for SRT
func SetSocketOptions(s C.int, binding int, options map[string]string) error {
    for _, so := range SocketOptions {
        if val, ok := options[so.name]; ok {
            if (so.binding == binding) {
                if (so.dataType == TInteger) {
                    v, err := strconv.Atoi(val)
                    if err == nil {
                        result := C.srt_setsockopt(s, 0, C.SRT_SOCKOPT(so.option), unsafe.Pointer(&v), C.int(unsafe.Sizeof(v)))
                        if result == -1 {
                            log.Printf("Warning - Error setting option %s to %s", so.name, val)
                        }
                    }
                } else if (so.dataType == TInteger64) {
                    v, err := strconv.ParseInt(val, 10, 64)
                    if err == nil {
                        result := C.srt_setsockopt(s, 0, C.SRT_SOCKOPT(so.option), unsafe.Pointer(&v), C.int(unsafe.Sizeof(v)))
                        if result == -1 {
                            log.Printf("Warning - Error setting option %s to %s", so.name, val)
                        }
                    }
                } else if (so.dataType == TString) {
                    sval := C.CString(val)
                    defer C.free(unsafe.Pointer(sval))
                    result := C.srt_setsockopt(s, 0, C.SRT_SOCKOPT(so.option), unsafe.Pointer(&sval), C.int(len(val)))
                    if result == -1 {
                        log.Printf("Warning - Error setting option %s to %s", so.name, val)
                    }

                } else if (so.dataType == TBoolean) {
                    var result C.int
                    if val == "1" {
                        v := C.char(1)
                        result = C.srt_setsockopt(s, 0, C.SRT_SOCKOPT(so.option), unsafe.Pointer(&v), C.int(unsafe.Sizeof(v)))
                    } else if val == "0" {
                        v := C.char(0)
                        result = C.srt_setsockopt(s, 0, C.SRT_SOCKOPT(so.option), unsafe.Pointer(&v), C.int(unsafe.Sizeof(v)))
                    }
                    if result == -1 {
                        log.Printf("Warning - Error setting option %s to %s", so.name, val)
                    }
                } else if (so.dataType == TTransType) {
                    var result C.int
                    if val == "live" {
                        v := TransTypeLive
                        result = C.srt_setsockopt(s, 0, C.SRT_SOCKOPT(so.option), unsafe.Pointer(&v), C.int(unsafe.Sizeof(v)))
                    } else if val == "file" {
                        v := TransTypeFile
                        result = C.srt_setsockopt(s, 0, C.SRT_SOCKOPT(so.option), unsafe.Pointer(&v), C.int(unsafe.Sizeof(v)))
                    }
                    if result == -1 {
                        log.Printf("Warning - Error setting option %s to %s", so.name, val)
                    }
                }
            }
        }
    }
	return nil
}