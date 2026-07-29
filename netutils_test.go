package srtgo

import (
	"testing"
)

// sa_data holds C `char`, whose signedness is ABI-defined: cgo maps it to int8
// where plain char is signed (x86-64 System V, Apple arm64) and to uint8 where
// it is unsigned (AArch64 Linux / AAPCS64). Expectations must therefore be
// written as raw bytes and compared through a byte() conversion, which is
// value-preserving in both directions for the same bit pattern.

func TestCreateAddrInetV4(t *testing.T) {
	ip1, size, err := CreateAddrInet("0.0.0.0", 8090)

	if err != nil {
		t.Error("Error on CreateAddrInet")
	}

	if size != 16 {
		t.Error("Ip Address size does not match", size)
	}

	if ip1.sa_family != afINET4 {
		t.Error("Ip Address family does not match")
	}

	// port 8090 == 0x1f9a, then 0.0.0.0
	data := []byte{0x1f, 0x9a, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	if len(data) != len(ip1.sa_data) {
		t.Error("Ip Address ip.sa_data length should be equal")
	}

	for i := 0; i < len(data); i++ {
		if data[i] != byte(ip1.sa_data[i]) {
			t.Errorf("Ip Address ip.sa_data does not match at %d: got %#02x, want %#02x", i, byte(ip1.sa_data[i]), data[i])
		}
	}

}

func TestCreateAddrInetV6(t *testing.T) {
	ip1, size, err := CreateAddrInet("2001:0db8:85a3:0000:0000:8a2e:0370:7334", 8090)

	if err != nil {
		t.Error("Error on CreateAddrInet")
	}

	if size != 28 {
		t.Error("Ipv6 Address size does not match", size)
	}

	if ip1.sa_family != afINET6 {
		t.Error("Ipv6 Address family does not match")
	}
	// port 8090 == 0x1f9a, zero flowinfo, then the leading 8 bytes of the address
	data := []byte{0x1f, 0x9a, 0x00, 0x00, 0x00, 0x00, 0x20, 0x01, 0x0d, 0xb8, 0x85, 0xa3, 0x00, 0x00}
	if len(data) != len(ip1.sa_data) {
		t.Error("Ipv6 Address ip.sa_data length should be equal")
	}

	for i := 0; i < len(data); i++ {
		if data[i] != byte(ip1.sa_data[i]) {
			t.Errorf("Ipv6 Address ip.sa_data does not match at %d: got %#02x, want %#02x", i, byte(ip1.sa_data[i]), data[i])
		}
	}

}
