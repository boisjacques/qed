package quic

import "net"

type direcionAddr uint8

const (
	local  = direcionAddr(0)
	remote = direcionAddr(1)
)

type Scheduler interface {
	Write([]byte) error
	Read([]byte) (int, net.Addr, error)
	Close() error
	LocalAddr() net.Addr
	RemoteAddr() net.Addr
	SetCurrentRemoteAddr(net.Addr)
	IsInitialised() bool
	SetIsInitialised(bool)
	IsActive() bool
	Activate(bool)
}

func xor(local, remote []byte) []byte {
	rval := make([]byte, 0)
	for i := 0; i < len(local); i++ {
		rval = append(rval, local[i])
	}
	for i := 0; i < len(remote); i++ {
		rval = append(rval, remote[i])
	}

	return rval
}

func isSameVersion(local, remote net.Addr) bool {
	if local.(*net.UDPAddr).IP.To4() == nil && remote.(*net.UDPAddr).IP.To4() == nil {
		return true
	}

	if local.(*net.UDPAddr).IP.To4() != nil && remote.(*net.UDPAddr).IP.To4() != nil {
		return true
	}
	return false
}
