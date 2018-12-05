package quic

import (
	"github.com/boisjacques/qed/internal/protocol"
	"github.com/boisjacques/qed/internal/wire"
	"time"
)

/*
This file defines a QED packet
It is a wrapper around QED control frames
 */

type Packet struct {
	PacketNumber    protocol.PacketNumber
	PacketType      protocol.PacketType
	Frames          []wire.Frame
	Length          protocol.ByteCount
	EncryptionLevel protocol.EncryptionLevel
	SendTime        time.Time
}
