package main

import (
	"crypto/sha256"
	"crypto/tls"
	"flag"
	"fmt"
	"github.com/boisjacques/qed"
	"log"
	"math/rand"
	"net"
	"os"
	"time"
)

func main() {
	var addr string
	var path string
	flag.StringVar(&addr, "addr", "", "address:port")
	flag.StringVar(&path, "path", "", "/path/to/file")

	flag.Parse()

	if addr == "" {
		interfaces, _ := net.Interfaces()
		for _, iface := range interfaces {
			if iface.Name == "en0" {
				addrs, err := iface.Addrs()
				if err != nil {
					fmt.Println(err)
					return
				}
				_addr := addrs[1].String()
				min3 := len(_addr) - 3
				addr = _addr[:min3] + ":4433"
			}
		}
	}

	var message []byte
	var messageLen uint64

	if path != "" {

		f, err := os.Open(path)
		if err != nil {
			panic(err)
		}
		defer f.Close()

		message = make([]byte, 0)
		f.Read(message)
	} else {
		messageLen = 100 * 1e6
		message = make([]byte, messageLen)
		rand.Seed(time.Now().UnixNano())
		rand.Read(message)
	}

	session, err := quic.DialAddr(addr, &tls.Config{InsecureSkipVerify: true}, nil)
	if err != nil {
		println(err.Error())
		return
	}

	stream, err := session.OpenStreamSync()
	if err != nil {
		println(err.Error())
		return
	}
	hasher := sha256.New()
	hasher.Write(message)
	sha := hasher.Sum(nil)
	log.Printf("SHA256 of message is %b", sha)

	var i uint64
	iteration := messageLen / 1000
	for i = 0; i < iteration; i++ {
		start := 1000 * i
		end := 1000 * (i+1)
		_, err = stream.Write(message[start:end])
		if err != nil {
			return
		}
	}


}
