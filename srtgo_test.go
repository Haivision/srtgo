package srtgo

import (
	"sync"
	"testing"
	"time"
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

func AcceptHelper(options map[string]string, t *testing.T) {
	listening := make(chan struct{})
	a := NewSrtSocket("localhost", 8090, options)
	b := NewSrtSocket("localhost", 8090, options)
	wg := sync.WaitGroup{}
	timer := time.AfterFunc(time.Second, func() {
		t.Log("Accept timed out")
		a.Close()
		b.Close()
	})
	wg.Add(1)
	go func() {
		<-listening
		err := b.Connect()
		if err != nil {
			t.Error(err)
		}
		wg.Done()
	}()

	err := a.Listen(1)
	if err != nil {
		t.Error(err)
	}
	listening <- struct{}{}
	sock, addr, err := a.Accept()
	if err != nil {
		t.Error(err)
	}
	if sock == nil || addr == nil {
		t.Error("Expected non-nil addr and sock")
	}

	wg.Wait()
	if timer.Stop() {
		a.Close()
		b.Close()
	}
}

func TestAcceptNonBlocking(t *testing.T) {
	InitSRT()

	options := make(map[string]string)
	options["transtype"] = "file"
	AcceptHelper(options, t)
}

func TestAcceptBlocking(t *testing.T) {
	InitSRT()

	options := make(map[string]string)
	options["blocking"] = "1"
	options["transtype"] = "file"
	AcceptHelper(options, t)
}

func TestSetSockOptInt(t *testing.T) {
	InitSRT()
	options := make(map[string]string)
	a := NewSrtSocket("localhost", 8090, options)

	expected := 200
	err := a.SetSockOptInt(SRTO_LATENCY, expected)
	if err != nil {
		t.Error(err)
	}

	v, err := a.GetSockOptInt(SRTO_LATENCY)
	if err != nil {
		t.Error(err)
	}
	if v != expected {
		t.Errorf("Failed to set SRTO_LATENCY expected %d, got %d\n", expected, v)
	}
}

func TestSetSockOptString(t *testing.T) {
	InitSRT()
	options := make(map[string]string)
	a := NewSrtSocket("localhost", 8090, options)

	expected := "123"
	err := a.SetSockOptString(SRTO_STREAMID, expected)
	if err != nil {
		t.Error(err)
	}

	v, err := a.GetSockOptString(SRTO_STREAMID)
	if err != nil {
		t.Error(err)
	}
	if v != expected {
		t.Errorf("Failed to set SRTO_STREAMID expected %s, got %s\n", expected, v)
	}
}

func TestSetSockOptBool(t *testing.T) {
	InitSRT()
	options := make(map[string]string)
	a := NewSrtSocket("localhost", 8090, options)

	expected := true
	err := a.SetSockOptBool(SRTO_MESSAGEAPI, expected)
	if err != nil {
		t.Error(err)
	}

	v, err := a.GetSockOptBool(SRTO_MESSAGEAPI)
	if err != nil {
		t.Error(err)
	}
	if v != expected {
		t.Errorf("Failed to set SRTO_MESSAGEAPI expected %t, got %t\n", expected, v)
	}
}
