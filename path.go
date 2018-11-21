package quic

import (
	"net"
)

// Defines a path between two endpoints
// It is part of QED
type Path struct {
	isPathZero bool
	pathID     uint32
	weight     int
	owd        uint64
	conn       *Conn
}

func NewPath(pathId uint32, pconn net.PacketConn, remote net.Addr, weight int) *Path {
	return &Path{
		isPathZero: false,
		pathID:     pathId,
		weight:     weight,
		owd:        0,
		conn:       NewConn(pconn, remote),
	}
}

func (p *Path) GetWeight() int {
	return p.weight
}

func (p *Path) GetPathID() uint32 {
	return p.pathID
}

func (p *Path) setOwd(owd int64) {
	p.owd = uint64(owd)
}

func (p *Path) GetConn() *Conn{
	return p.conn
}

func (p *Path) contains(address net.Addr) bool {
	return (p.conn.GetLocal() == address || p.conn.GetRemote() == address)
}
