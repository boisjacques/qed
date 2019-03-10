package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"github.com/boisjacques/qed"
	"io"
	"os"
)

func main() {
	var addr string
	var path string
	flag.StringVar(&addr, "addr", "", "address:port")
	flag.StringVar(&path, "path", "", "/path/to/file")

	flag.Parse()

	if addr == "" {
		fmt.Println("no address provided")
		os.Exit(-1)
	}

	file, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	session, err := quic.DialAddr(addr, &tls.Config{InsecureSkipVerify: true}, nil)
	if err != nil {
		panic(err.Error())
	}

	stream, err := session.OpenStreamSync()
	if err != nil {
		panic(err.Error())
	}

	sendBuffer := make([]byte, 512)

	for {
		_, err = file.Read(sendBuffer)
		if err == io.EOF {
			break
		}
		_, sendError := stream.Write(sendBuffer)
		if sendError != nil {
			println(sendError)
		}
	}
	err = nil
	err = stream.Close()
	if err != nil {
		panic(err)
	}

}
