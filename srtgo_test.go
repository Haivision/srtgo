package srtgo

import (
	"testing"
)

func TestNewSocket(t *testing.T) {
	options := make(map[string]string)
	a := NewSrtSocket("localhost", 8090, options)

	if a == nil {
		t.Error("Could not create a srt socket")
	}
}

func TestNewSocketBlocking(t *testing.T) {
	options := make(map[string]string)
	options["blocking"] = "true"
	a := NewSrtSocket("localhost", 8090, options)

	if a == nil {
		t.Error("Could not create a srt socket")
	}
}

func TestNewSocketLinger(t *testing.T) {
	options := make(map[string]string)
	options["linger"] = "1000"
	a := NewSrtSocket("localhost", 8090, options)

	if a == nil {
		t.Error("Could not create a srt socket")
	}
}

func TestNewSocketWithTransType(t *testing.T) {
	options := make(map[string]string)
	options["transtype"] = "3"
	a := NewSrtSocket("localhost", 8090, options)

	if a == nil {
		t.Error("Could not create a srt socket")
	}
}

func TestNewSocketWithParameters(t *testing.T) {
	options := make(map[string]string)
	options["pbkeylen"] = "32"
	a := NewSrtSocket("localhost", 8090, options)

	if a == nil {
		t.Error("Could not create a srt socket")
	}
}

func TestNewSocketWithInt64Param(t *testing.T) {
	options := make(map[string]string)
	options["maxbw"] = "300000"
	a := NewSrtSocket("localhost", 8090, options)

	if a == nil {
		t.Error("Could not create a srt socket")
	}
}

func TestNewSocketWithBoolParam(t *testing.T) {
	options := make(map[string]string)
	options["enforcedencryption"] = "0"
	a := NewSrtSocket("localhost", 8090, options)

	if a == nil {
		t.Error("Could not create a srt socket")
	}
}

func TestNewSocketWithStringParam(t *testing.T) {
	options := make(map[string]string)
	options["passphrase"] = "11111111111"
	a := NewSrtSocket("localhost", 8090, options)

	if a == nil {
		t.Error("Could not create a srt socket")
	}
}

func TestListen(t *testing.T) {
	InitSRT()

	options := make(map[string]string)
	options["blocking"] = "0"
	options["transtype"] = "file"

	a := NewSrtSocket("0.0.0.0", 8090, options)
	err := a.Listen(2)
	if err != nil {
		t.Error("Error on testListen")
	}
}

func TestSetSockOptInt(t *testing.T) {
	InitSRT()
	options := make(map[string]string)
	a := NewSrtSocket("localhost", 8090, options)

	err := a.SetSockOptInt(16, 20000)
	if err != nil {
		t.Error("Error on TestSetSockOpt")
	}

	v, err := a.GetSockOptInt(16)
	if v != 20000 {
		t.Error("Error in SetSockOptInt/GetSockOptInt", v)
	}
}

func TestSetSockOptString(t *testing.T) {
	InitSRT()
	options := make(map[string]string)
	a := NewSrtSocket("localhost", 8090, options)

	err := a.SetSockOptString(46, "123")
	if err != nil {
		t.Error("Error on TestSetSockOpt")
	}

	v, err := a.GetSockOptString(46)
	if v != "123" {
		t.Error("Error in SetSockOptString/GetSockOptString", v)
	}
}

func TestSetSockOptBool(t *testing.T) {
	InitSRT()
	options := make(map[string]string)
	a := NewSrtSocket("localhost", 8090, options)

	err := a.SetSockOptBool(48, true)
	if err != nil {
		t.Error("Error on TestSetSockOpt")
	}

	v, err := a.GetSockOptBool(48)
	if v != true {
		t.Error("Error in SetSockOptBool/GetSockOptBool", v)
	}
}
