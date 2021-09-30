// build windows

package srtgo

import (
	"golang.org/x/sys/windows"
	"unsafe"
)

const (
	afINET4 = windows.AF_INET
	afINET6 = windows.AF_INET6
)

var (
	sizeofSockAddrInet4 = 0
	sizeofSockAddrInet6 = 0
	sizeofSockaddrAny   = 0
)

func init() {
	var inet4 windows.RawSockaddrInet4
	var inet6 windows.RawSockaddrInet6
	var any windows.RawSockaddrAny
	sizeofSockAddrInet4 = unsafe.Sizeof(inet4)
	sizeofSockAddrInet6 = unsafe.Sizeof(inet6)
	sizeofSockaddrAny = unsafe.Sizeof(any)
}
