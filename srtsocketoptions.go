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
    transTypeLive = 0
    transTypeFile = 1
)

const (
	tInteger   = 0
	tInteger64 = 1
	tString    = 2
    tBoolean   = 3
    tTransType = 4
)

type socketOption struct {
	name       string
	level      int
	option	   int
	binding    int
	dataType   int
}

// List of possible srt socket options
var SocketOptions = []socketOption{
	{ "transtype", 0, C.SRTO_TRANSTYPE, bindingPre, tTransType },
	{ "maxbw", 0, C.SRTO_MAXBW, bindingPre, tInteger64 },
    { "pbkeylen", 0, C.SRTO_PBKEYLEN, bindingPre, tInteger },
    { "passphrase", 0, C.SRTO_PASSPHRASE, bindingPre, tString },
    { "mss", 0, C.SRTO_MSS, bindingPre, tInteger },
    { "fc", 0, C.SRTO_FC, bindingPre, tInteger },
    { "sndbuf", 0, C.SRTO_SNDBUF, bindingPre, tInteger },
    { "rcvbuf", 0, C.SRTO_RCVBUF, bindingPre, tInteger },
    { "ipttl", 0, C.SRTO_IPTTL, bindingPre, tInteger },
    { "iptos", 0, C.SRTO_IPTOS, bindingPre, tInteger },
    { "inputbw", 0, C.SRTO_INPUTBW, bindingPost, tInteger64 },
    { "oheadbw", 0, C.SRTO_OHEADBW, bindingPost, tInteger },
    { "latency", 0, C.SRTO_LATENCY, bindingPre, tInteger },
    { "tsbpdmode", 0, C.SRTO_TSBPDMODE, bindingPre, tBoolean },
    { "tlpktdrop", 0, C.SRTO_TLPKTDROP, bindingPre, tBoolean },
    { "snddropdelay", 0, C.SRTO_SNDDROPDELAY, bindingPost, tInteger },
    { "nakreport", 0, C.SRTO_NAKREPORT, bindingPre, tBoolean },
    { "conntimeo", 0, C.SRTO_CONNTIMEO, bindingPre, tInteger },
    { "lossmaxttl", 0, C.SRTO_LOSSMAXTTL, bindingPre, tInteger },
    { "rcvlatency", 0, C.SRTO_RCVLATENCY, bindingPre, tInteger },
    { "peerlatency", 0, C.SRTO_PEERLATENCY, bindingPre, tInteger },
    { "minversion", 0, C.SRTO_MINVERSION, bindingPre, tInteger },
    { "streamid", 0, C.SRTO_STREAMID, bindingPre, tString },
    { "congestion", 0, C.SRTO_CONGESTION, bindingPre, tString },
    { "messageapi", 0, C.SRTO_MESSAGEAPI, bindingPre, tBoolean },
    { "payloadsize", 0, C.SRTO_PAYLOADSIZE, bindingPre, tInteger },
    { "kmrefreshrate", 0, C.SRTO_KMREFRESHRATE, bindingPre, tInteger },
    { "kmpreannounce", 0, C.SRTO_KMPREANNOUNCE, bindingPre, tInteger },
    { "enforcedencryption", 0, C.SRTO_ENFORCEDENCRYPTION, bindingPre, tBoolean },
    { "peeridletimeo", 0, C.SRTO_PEERIDLETIMEO, bindingPre, tInteger },
    { "packetfilter", 0, C.SRTO_PACKETFILTER, bindingPre, tString },
}

func setSocketLingerOption(s C.int, li int32) error {
	var lin syscall.Linger;
	lin.Linger = li;
	if (lin.Linger > 0 ) {
		lin.Onoff = 1
	} else {
		lin.Onoff = 0
	}
	// C.srt_setsockopt(s.socket, bindingPre, C.SRTO_LINGER, unsafe.Pointer(&lin), C.int(unsafe.Sizeof(lin)))
	return syscall.SetsockoptLinger(int(s), bindingPre, syscall.SO_LINGER, &lin)
}

// Set socket options for SRT
func setSocketOptions(s C.int, binding int, options map[string]string) error {
    for _, so := range SocketOptions {
        if val, ok := options[so.name]; ok {
            if (so.binding == binding) {
                if (so.dataType == tInteger) {
                    v, err := strconv.Atoi(val)
                    if err == nil {
                        result := C.srt_setsockopt(s, 0, C.SRT_SOCKOPT(so.option), unsafe.Pointer(&v), C.int(unsafe.Sizeof(v)))
                        if result == -1 {
                            log.Printf("Warning - Error setting option %s to %s", so.name, val)
                        }
                    }
                } else if (so.dataType == tInteger64) {
                    v, err := strconv.ParseInt(val, 10, 64)
                    if err == nil {
                        result := C.srt_setsockopt(s, 0, C.SRT_SOCKOPT(so.option), unsafe.Pointer(&v), C.int(unsafe.Sizeof(v)))
                        if result == -1 {
                            log.Printf("Warning - Error setting option %s to %s", so.name, val)
                        }
                    }
                } else if (so.dataType == tString) {
                    sval := C.CString(val)
                    defer C.free(unsafe.Pointer(sval))
                    result := C.srt_setsockopt(s, 0, C.SRT_SOCKOPT(so.option), unsafe.Pointer(&sval), C.int(len(val)))
                    if result == -1 {
                        log.Printf("Warning - Error setting option %s to %s", so.name, val)
                    }

                } else if (so.dataType == tBoolean) {
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
                } else if (so.dataType == tTransType) {
                    var result C.int
                    if val == "live" {
                        v := transTypeLive
                        result = C.srt_setsockopt(s, 0, C.SRT_SOCKOPT(so.option), unsafe.Pointer(&v), C.int(unsafe.Sizeof(v)))
                    } else if val == "file" {
                        v := transTypeFile
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