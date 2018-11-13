package quic

import (
	"github.com/boisjacques/qed/internal/protocol"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Buffer Pool", func() {
	It("returns buffers of cap", func() {
		buf := *getPacketBuffer()
		Expect(buf).To(HaveCap(int(protocol.MaxReceivePacketSize)))
	})

	It("panics if wrong-sized buffers are passed", func() {
		Expect(func() {
			putPacketBuffer(&[]byte{0})
		}).To(Panic())
	})
})
