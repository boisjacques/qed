package main

import (
	"crypto/sha256"
	"crypto/tls"
	"flag"
	"fmt"
	"github.com/boisjacques/qed"
	"math/rand"
	"os"
	"time"
)

func main() {
	var addr string
	var path string
	flag.StringVar(&addr, "addr", "localhost:4433", "address:port")
	flag.StringVar(&path, "path", "", "/path/to/file")

	var message []byte

	if path != "" {

		f, err := os.Open(path)
		if err != nil {
			panic(err)
		}
		defer f.Close()

		message = make([]byte, 0)
		f.Read(message)
	} else {
		messageLen := 100 * 1e6
		message = make([]byte, messageLen)
		rand.Seed(time.Now().UnixNano())
		rand.Read(message)
	}

	session, err := quic.DialAddr(addr, &tls.Config{InsecureSkipVerify: true}, nil)
	if err != nil {
		return
	}

	stream, err := session.OpenStreamSync()
	if err != nil {
		return
	}
	hasher := sha256.New()
	hasher.Write(message)
	sha := string(hasher.Sum(nil))
	fmt.Println("SHA256 of message is" + sha)
	_, err = stream.Write(message)
	if err != nil {
		return
	}

}
