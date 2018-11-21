package quic

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Client Multiplexer", func() {
	It("adds a new packet Conn ", func() {
		conn := newMockPacketConn()
		_, err := getMultiplexer().AddConn(conn, 8)
		Expect(err).ToNot(HaveOccurred())
	})

	It("errors when adding an existing Conn with a different Connection ID length", func() {
		conn := newMockPacketConn()
		_, err := getMultiplexer().AddConn(conn, 5)
		Expect(err).ToNot(HaveOccurred())
		_, err = getMultiplexer().AddConn(conn, 6)
		Expect(err).To(MatchError("cannot use 6 byte Connection IDs on a Connection that is already using 5 byte connction IDs"))
	})

})
