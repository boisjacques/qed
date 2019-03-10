package wire

import (
	"bytes"
	"github.com/boisjacques/qed/internal/protocol"
	"github.com/boisjacques/qed/internal/utils"
)

// An OwdAckFrame is a QED OWD Ack Frame
// It is part of the QED extension
type OwdAckFrame struct {
	pathID uint32
	owd    uint64
}

func (o *OwdAckFrame) Owd() uint64 {
	return o.owd
}

func (o *OwdAckFrame) PathID() uint32 {
	return o.pathID
}

func NewOwdAckFrame(pathId uint32, owd uint64) *OwdAckFrame {
	return &OwdAckFrame{
		pathID: pathId,
		owd:    owd,
	}
}

func parseOwdAckFrame(r *bytes.Reader, version protocol.VersionNumber) (*OwdAckFrame, error) {
	// Eliminate typeByte
	if _, err := r.ReadByte(); err != nil {
		return nil, err
	}

	var pathID uint32
	var owd uint64

	pi, err := utils.ReadVarInt(r)
	if err != nil {
		return nil, err
	}
	pathID = uint32(pi)

	t, err := utils.ReadVarInt(r)
	if err != nil {
		return nil, err
	}
	owd = uint64(t)

	return &OwdAckFrame{
		pathID: pathID,
		owd:    owd,
	}, nil
}

func (f *OwdAckFrame) Write(b *bytes.Buffer, version protocol.VersionNumber) error {
	typeByte := byte(0xf2)
	b.WriteByte(typeByte)
	utils.WriteVarInt(b, uint64(f.pathID))
	utils.WriteVarInt(b, uint64(f.owd))
	return nil
}

func (f *OwdAckFrame) Length(version protocol.VersionNumber) protocol.ByteCount {
	length := utils.VarIntLen(uint64(f.pathID))
	length += utils.VarIntLen(uint64(f.owd))
	return length
}
