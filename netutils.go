package srtgo

import "C"

import (
	"fmt"
	"net"
	"syscall"
	"unsafe"
)

func sockAddrFromIp4(ip net.IP, port uint16) (*C.struct_sockaddr, int, error) {
	var raw syscall.RawSockaddrInet4
	raw.Len = syscall.SizeofSockaddrInet4
	raw.Family = syscall.AF_INET

	p := (*[2]byte)(unsafe.Pointer(&raw.Port))
	p[0] = byte(port >> 8)
	p[1] = byte(port)

	copy(raw.Addr[:], ip.To4())

	return (*C.struct_sockaddr)(unsafe.Pointer(&raw)), int(raw.Len), nil
}

func sockAddrFromIp6(ip net.IP, port uint16) (*C.struct_sockaddr, int, error) {
	var raw syscall.RawSockaddrInet6
	raw.Len = syscall.SizeofSockaddrInet6
	raw.Family = syscall.AF_INET6

	p := (*[2]byte)(unsafe.Pointer(&raw.Port))
	p[0] = byte(port >> 8)
	p[1] = byte(port)

	copy(raw.Addr[:], ip.To16())

	return (*C.struct_sockaddr)(unsafe.Pointer(&raw)), int(raw.Len), nil
}

func CreateAddrInet(name string, port uint16) (*C.struct_sockaddr, int, error) {
	ip := net.ParseIP(name)
	if ip == nil {
		ips, err := net.LookupIP(name)
		if err != nil {
			return nil, 0, fmt.Errorf("Error in CreateAddrInet, LookupIP")
		}
		ip = ips[0]
	}

	if ip.To4() != nil {
		return sockAddrFromIp4(ip, port)
	} else if ip.To16() != nil {
		return sockAddrFromIp6(ip, port)
	}

	return nil, 0, fmt.Errorf("Error in CreateAddrInet, LookupIP")
}
