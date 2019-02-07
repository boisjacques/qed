package main

import (
	"bufio"
	"flag"
	"fmt"
	"github.com/tylerwince/godbg"
	"net"
	"os"
)

var addr string

func main(){
	flag.StringVar(&addr, "addr", "", "address:port")
	flag.Parse()
	if addr == "" {
		godbg.Dbg("No address provided. Exiting.")
		return
	}
	genericSock,err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4zero, Port: 4711})
	if err != nil {
		godbg.Dbg(err)
	}
	err = nil
	udpaddr,err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		godbg.Dbg(err)
	}
	err = nil
	usock, err := net.ListenUDP("udp", udpaddr)
	if err != nil {
		godbg.Dbg(err)
	}


	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Press any key to exit…")
	reader.ReadString('\n')

	genericSock.Close()
	usock.Close()
}
