package srtgo

import (
	"testing"
)

func TestCreateAddrInetV4(t *testing.T) {
	ip1, size, err := CreateAddrInet("0.0.0.0", 8090)

	if err != nil {
		t.Error("Error on CreateAddrInet")
	}

	if size != 16 {
		t.Error("Ip Address size does not match", size)
	}

	if ip1.sa_family != 2 {
		t.Error("Ip Address family does not match")
	}

	data := []int{31, -102, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	if len(data) != len(ip1.sa_data) {
		t.Error("Ip Address ip.sa_data length should be equal")
	}

	for i := 0; i < len(data); i++ {
		if data[i] != int(ip1.sa_data[i]) {
			t.Error("Ip Address ip.sa_data does not match")
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

	if ip1.sa_family != 30 {
		t.Error("Ipv6 Address family does not match")
	}
	data := []int{31, -102, 0, 0, 0, 0, 32, 1, 13, -72, -123, -93, 0, 0}
	if len(data) != len(ip1.sa_data) {
		t.Error("Ipv6 Address ip.sa_data length should be equal")
	}

	for i := 0; i < len(data); i++ {
		if data[i] != int(ip1.sa_data[i]) {
			t.Error("Ipv6 Address ip.sa_data does not match")
		}
	}

}
