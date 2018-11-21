package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"github.com/boisjacques/qed"
	"os"
)

func main() {
	var addr string
	var path string
	flag.StringVar(&addr, "addr", "localhost:4433", "address:port")
	flag.StringVar(&path, "path", "", "/path/to/file")

	f, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	message := make([]byte, 0)
	f.Read(message)

	session, err := quic.DialAddr(addr, &tls.Config{InsecureSkipVerify: true}, nil)
	if err != nil {
		return
	}

	stream, err := session.OpenStreamSync()
	if err != nil {
		return
	}

	fmt.Printf("Client: Sending '%s'\n", message)
	_, err = stream.Write(message)
	if err != nil {
		return
	}


}
