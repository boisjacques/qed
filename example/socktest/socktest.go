package main

import (
	"flag"
	"github.com/tylerwince/godbg"
	"net"
)

var addr string

func main(){
	flag.StringVar(&addr, "addr", "", "address:port")
	flag.Parse()
	if addr == "" {
		godbg.Dbg("No address provided. Exiting.")
		return
	}
	genericSock,err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
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
	genericSock.Close()
	usock.Close()
}
