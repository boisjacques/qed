package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"github.com/boisjacques/qed"
	"io"
	"log"
)

var addr string
const message = "foobar"

// Adapted example echo client/server from quic-go
func main() {
	flag.StringVar(&addr, "addr", "", "address:port")
	flag.Parse()
	if addr == "" {
		log.Fatal("No address provided. Exiting.")
		return
	}
	err := clientMain()
	if err != nil {
		panic(err)
	}
}



func clientMain() error {
	session, err := quic.DialAddr(addr, &tls.Config{InsecureSkipVerify: true}, nil)
	if err != nil {
		return err
	}

	stream, err := session.OpenStreamSync()
	if err != nil {
		return err
	}

	fmt.Printf("Client: Sending '%s'\n", message)
	_, err = stream.Write([]byte(message))
	if err != nil {
		return err
	}

	buf := make([]byte, len(message))
	_, err = io.ReadFull(stream, buf)
	if err != nil {
		return err
	}
	fmt.Printf("Client: Got '%s'\n", buf)

	return nil
}

