package wire

import (
	"bytes"

	"github.com/boisjacques/qed/internal/protocol"
)

// A Frame in QUIC
type Frame interface {
	Write(b *bytes.Buffer, version protocol.VersionNumber) error
	Length(version protocol.VersionNumber) protocol.ByteCount
}
