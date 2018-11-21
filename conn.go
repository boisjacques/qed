package quic

import (
	"net"
	"sync"
)

type Connection interface {
	Write([]byte) error
	Read([]byte) (int, net.Addr, error)
	Close() error
	LocalAddr() net.Addr
	RemoteAddr() net.Addr
	SetCurrentRemoteAddr(net.Addr)
}

type Conn struct {
	mutex sync.RWMutex

	pconn       net.PacketConn
	currentAddr net.Addr
}

func NewConn(pconn net.PacketConn, remote net.Addr) *Conn {
	return &Conn{pconn: pconn, currentAddr: remote}
}

var _ Connection = &Conn{}

func (c *Conn) Write(p []byte) error {
	_, err := c.pconn.WriteTo(p, c.currentAddr)
	return err
}

func (c *Conn) Read(p []byte) (int, net.Addr, error) {
	return c.pconn.ReadFrom(p)
}

func (c *Conn) SetCurrentRemoteAddr(addr net.Addr) {
	c.mutex.Lock()
	c.currentAddr = addr
	c.mutex.Unlock()
}

func (c *Conn) LocalAddr() net.Addr {
	return c.pconn.LocalAddr()
}

func (c *Conn) RemoteAddr() net.Addr {
	c.mutex.RLock()
	addr := c.currentAddr
	c.mutex.RUnlock()
	return addr
}

func (c *Conn) Close() error {
	return c.pconn.Close()
}

func (c *Conn) GetLocal() net.Addr {
	return c.pconn.LocalAddr()
}

func (c *Conn) GetRemote() net.Addr {
	return c.RemoteAddr()
}