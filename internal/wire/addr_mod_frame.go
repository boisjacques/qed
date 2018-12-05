package wire

import (
	"bytes"
	"github.com/boisjacques/qed/internal/protocol"
	"github.com/boisjacques/qed/internal/utils"
	"io"
	"net"
)

type AddressModificationOperation uint8

const (
	DeleteFrame = AddressModificationOperation(0x0)
	AddFrame    = AddressModificationOperation(0x1)
)

type IpVersion uint8

const (
	IPv4 = IpVersion(0x4)
	IPv6 = IpVersion(0x6)
)

// An AddrModFrame is a QED Address Modification Frame
// It is part of the QED extension
type AddrModFrame struct {
	operation      AddressModificationOperation
	addressVersion IpVersion
	address        net.Addr
}

func (a *AddrModFrame) Address() net.Addr {
	return a.address
}

func (a *AddrModFrame) AddressVersion() IpVersion {
	return a.addressVersion
}

func (a *AddrModFrame) Operation() AddressModificationOperation {
	return a.operation
}

func NewAddrModFrame(operation AddressModificationOperation, version IpVersion, addr net.Addr) *AddrModFrame {
	return &AddrModFrame{
		operation:      operation,
		addressVersion: version,
		address:        addr.(*net.UDPAddr),
	}
}

func parseAddrModFrame(r *bytes.Reader, version protocol.VersionNumber) (*AddrModFrame, error) {
	// Eliminate typeByte
	if _, err := r.ReadByte(); err != nil {
		return nil, err
	}
	var modType AddressModificationOperation
	var addressVersion IpVersion
	var address net.Addr

	mt, err := utils.ReadVarInt(r)
	if err != nil {
		return nil, err
	}
	modType = AddressModificationOperation(mt)

	av, err := utils.ReadVarInt(r)
	if err != nil {
		return nil, err
	}
	addressVersion = IpVersion(av)

	addr := make([]byte, 0)
	if _, err := io.ReadFull(r, addr); err != nil {
		return nil, err
	}
	address, err = net.ResolveUDPAddr("udp", string(addr))
	if err != nil {
		return nil, err
	}

	return &AddrModFrame{
		operation:      modType,
		addressVersion: addressVersion,
		address:        address,
	}, nil
}

func (f *AddrModFrame) Write(b *bytes.Buffer, version protocol.VersionNumber) error {
	typeByte := byte(0xf1)
	b.WriteByte(typeByte)
	utils.WriteVarInt(b, uint64(f.operation))
	utils.WriteVarInt(b, uint64(f.addressVersion))
	b.Write([]byte(f.address.String()))
	return nil
}

func (f *AddrModFrame) Length(version protocol.VersionNumber) protocol.ByteCount {
	length := utils.VarIntLen(uint64(f.operation))
	length += utils.VarIntLen(uint64(f.addressVersion))
	addrArray := []byte(f.address.String())
	for b := range addrArray {
		length += utils.VarIntLen(uint64(b))
	}
	return length
}
