package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"flag"
	"fmt"
	"github.com/boisjacques/qed"
	"io/ioutil"
	"log"
	"math/big"
	"os"
	"time"
)

func main() {
	var addr string
	var path string
	flag.StringVar(&addr, "addr", "", "address:port")
	flag.StringVar(&path, "path", "out.file", "/path/to/file")

	flag.Parse()

	if addr == "" {
		fmt.Println("no address provided")
		os.Exit(-1)
	}

	listener, err := quic.ListenAddr(addr, generateTLSConfig(), nil)
	if err != nil {
		panic(err)
	}

	if err != nil {
		panic(err)
	}
	for {
		sess, err := listener.Accept()
		if err != nil {
			panic(err)
		}

		go handleSession(sess, path)
	}
}

func handleSession(sess quic.Session, path string) {
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	stream, err := sess.AcceptStream()
	if err != nil {
		panic(err)
	}
	recv := make([]byte, 0)
	for {
		if _, err := stream.Read(recv); err != nil {
			log.Fatalf("error reading from stream %s: %s", stream.StreamID().StreamNum(), err)
			panic(err)
		}
	}
	hasher := sha256.New()
	hasher.Write(recv)
	sha := hex.EncodeToString(hasher.Sum(nil))
	log.Printf("SHA256 of message is %b", sha)

	if err := ioutil.WriteFile(path, recv, 0644); err != nil {
		log.Fatalf("error writing file: %s", err)
		panic(err)
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
	keyFileName := "key-" + time.Now().String() + ".pem"
	keyOut, err := os.OpenFile(keyFileName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Printf("failed to open %s for writing: %s", keyFileName, err)
		panic(err)
	}
	if _, err := keyOut.Write(keyPEM); err != nil {
		log.Fatalf("error writing key: %s", err)
	}
	if err := keyOut.Close(); err != nil {
		log.Fatalf("error closing %s: %s", keyFileName, err)
	}
	log.Printf("wrote %s\n", keyFileName)

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		panic(err)
	}
	return &tls.Config{Certificates: []tls.Certificate{tlsCert}}
}
