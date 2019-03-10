package wire

import (
	"bytes"
	"github.com/boisjacques/qed/internal/protocol"
	"github.com/boisjacques/qed/internal/utils"
	"time"
)

// An OwdFrame is a QED OWD Frame
// It is part of the QED extension
type OwdFrame struct {
	pathID uint32
	time   uint64
}

func (o *OwdFrame) Time() uint64 {
	return o.time
}

func (o *OwdFrame) PathID() uint32 {
	return o.pathID
}

func parseOwdFrame(r *bytes.Reader, version protocol.VersionNumber) (*OwdFrame, error) {
	// Eliminate typeByte
	if _, err := r.ReadByte(); err != nil {
		return nil, err
	}

	var pathID uint32
	var time uint64

	pi, err := utils.ReadVarInt(r)
	if err != nil {
		return nil, err
	}
	pathID = uint32(pi)

	t, err := utils.ReadVarInt(r)
	if err != nil {
		return nil, err
	}
	time = uint64(t)

	return &OwdFrame{
		pathID: pathID,
		time:   time,
	}, nil
}

func NewOwdFrame(pathId uint32) *OwdFrame {
	return &OwdFrame{
		pathID: pathId,
		time:   uint64(time.Now().Unix()),
	}
}

func (f *OwdFrame) Write(b *bytes.Buffer, version protocol.VersionNumber) error {
	typeByte := byte(0xf2)
	b.WriteByte(typeByte)
	utils.WriteVarInt(b, uint64(f.pathID))
	utils.WriteVarInt(b, uint64(f.time))
	return nil
}

func (f *OwdFrame) Length(version protocol.VersionNumber) protocol.ByteCount {
	length := utils.VarIntLen(uint64(f.pathID))
	length += utils.VarIntLen(uint64(f.time))
	return length
}
