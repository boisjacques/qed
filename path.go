package quic

import (
	"fmt"
	"github.com/VividCortex/ewma"
	"net"
)

// Defines a path between two endpoints
// It is part of QED
type Path struct {
	isPathZero bool
	pathID     uint32
	weight     int
	owd        float64
	local      net.PacketConn
	remote     net.Addr
	vewma      ewma.MovingAverage
}

func NewPath(pathId uint32, pconn net.PacketConn, remote net.Addr, weight int) *Path {
	return &Path{
		isPathZero: false,
		pathID:     pathId,
		weight:     weight,
		owd:        0,
		local:      pconn,
		remote:     remote,
		vewma:      ewma.NewMovingAverage(5),
	}
}

func (p *Path) GetWeight() int {
	return p.weight
}

func (p *Path) GetPathID() uint32 {
	return p.pathID
}

func (p *Path) setOwd(owd float64) {
	p.vewma.Add(owd)
	p.owd = owd
}

func (p *Path) contains(address net.Addr) bool {
	return (p.local.LocalAddr() == address || p.remote == address)
}

func (p *Path) Write() string {
	return fmt.Sprintf("%d\n%s\n%s", p.pathID, p.local.LocalAddr().String(), p.remote.String())
}

func (p *Path) MovingAverage() float64 {
	if p.vewma.Value() == 0.0 {
		return p.owd
	}
	return p.vewma.Value()
}
