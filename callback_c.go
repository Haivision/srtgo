package srtgo

/*
#cgo LDFLAGS: -lsrt
#include <srt/srt.h>

extern void srtListenCBWrapper(void* opaque, SRTSOCKET ns, int hs_version, struct sockaddr* peeraddr, char* streamid);

void srtListenCB(void* opaque, SRTSOCKET ns, int hs_version, const struct sockaddr* peeraddr, const char* streamid)
{
	srtListenCBWrapper(opaque, ns, hs_version, (struct sockaddr*)peeraddr, (char*)streamid);
}
*/
import "C"
