package srtgo

type SrtInvalidSock struct{}
type SrtRendezvousUnbound struct{}
type SrtSockConnected struct{}
type SrtConnectionRejected struct{}
type SrtConnectTimeout struct{}
type SrtSocketClosed struct{}
type SrtEpollTimeout struct{}

func (m *SrtInvalidSock) Error() string {
	return "Socket u indicates no valid socket ID"
}

func (m *SrtRendezvousUnbound) Error() string {
	return "Socket u is in rendezvous mode, but it wasn't bound"
}

func (m *SrtSockConnected) Error() string {
	return "Socket u is already connected"
}

func (m *SrtConnectionRejected) Error() string {
	return "Connection has been rejected"
}

func (m *SrtConnectTimeout) Error() string {
	return "Connection has been timed out"
}

func (m *SrtSocketClosed) Error() string {
	return "The socket has been closed"
}

func (m *SrtEpollTimeout) Error() string {
	return "Operation has timed out"
}

func (m *SrtEpollTimeout) Timeout() bool {
	return true
}

func (m *SrtEpollTimeout) Temporary() bool {
	return true
}
