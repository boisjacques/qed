package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"flag"
	"fmt"
	"github.com/boisjacques/qed"
	"log"
	"math/big"
	"os"
	"time"
)

func main() {
	var addr string
	var path string
	flag.StringVar(&addr, "addr", "0.0.0.0:4433", "address:port")
	flag.StringVar(&path, "path", "out.file", "/path/to/file")

	flag.Parse()

	listener, err := quic.ListenAddr(addr, generateTLSConfig(), nil)

	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	if err != nil {
		return
	}
	sess, err := listener.Accept()
	if err != nil {
		return
	}
	stream, err := sess.AcceptStream()
	if err != nil {
		panic(err)
	}
	recv := make([]byte, 0)
	if _,err := stream.Read(recv); err != nil {
		log.Fatalf("error receiving packets: %s", err)
	}
	log.Printf("server shut down at: %s", time.Now())
	hasher := sha256.New()
	hasher.Write(recv)
	sha := string(hasher.Sum(nil))
	fmt.Println("SHA256 of message is " + sha)

	if _,err := f.Write(recv); err != nil {
		log.Fatalf("error writing file: %s", err)
	}
}

// Setup a bare-bones TLS config for the server
// Copied from quic-go
func generateTLSConfig() *tls.Config {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		panic(err)
	}
	template := x509.Certificate{SerialNumber: big.NewInt(1)}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		panic(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	// Copied from golang.org language documentation
	keyOut, err := os.OpenFile("key.pem", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Print("failed to open key.pem for writing:", err)
		panic(err)
	}
	if _,err := keyOut.Write(keyPEM); err != nil {
		log.Fatalf("error writing key: %s", err)
	}
	if err := keyOut.Close(); err != nil {
		log.Fatalf("error closing key.pem: %s", err)
	}
	log.Print("wrote key.pem\n")

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		panic(err)
	}
	return &tls.Config{Certificates: []tls.Certificate{tlsCert}}
}
