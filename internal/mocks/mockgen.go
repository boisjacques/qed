package mocks

//go:generate sh -c "../mockgen_internal.sh mocks sealer.go github.com/boisjacques/qed/internal/handshake Sealer"
//go:generate sh -c "../mockgen_internal.sh mocks crypto_setup.go github.com/boisjacques/qed/internal/handshake CryptoSetup"
//go:generate sh -c "../mockgen_internal.sh mocks stream_flow_controller.go github.com/boisjacques/qed/internal/flowcontrol StreamFlowController"
//go:generate sh -c "../mockgen_internal.sh mockackhandler ackhandler/sent_packet_handler.go github.com/boisjacques/qed/internal/ackhandler SentPacketHandler"
//go:generate sh -c "../mockgen_internal.sh mockackhandler ackhandler/received_packet_handler.go github.com/boisjacques/qed/internal/ackhandler ReceivedPacketHandler"
//go:generate sh -c "../mockgen_internal.sh mocks congestion.go github.com/boisjacques/qed/internal/congestion SendAlgorithm"
//go:generate sh -c "../mockgen_internal.sh mocks connection_flow_controller.go github.com/boisjacques/qed/internal/flowcontrol ConnectionFlowController"
//go:generate sh -c "../mockgen_internal.sh mockcrypto crypto/aead.go github.com/boisjacques/qed/internal/crypto AEAD"
