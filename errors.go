package srtgo

type SrtInvalidSock struct{}
type SrtRendezvousUnbound struct{}
type SrtSockConnected struct{}
type SrtConnectionRejected struct{}
type SrtConnectTimeout struct{}
type SrtSocketClosed struct{}

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
	return "The socket u has been closed while the function was blocking the call"
}
