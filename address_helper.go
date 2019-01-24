package quic

import (
	"github.com/tylerwince/godbg"
	"hash/crc32"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

type AddressHelper struct {
	ipAddresses  map[uint32]net.Addr
	listeners    []chan map[uint32]net.Addr
	isInitalised bool
}

var addrHlp *AddressHelper
var once sync.Once

func GetAddressHelper() *AddressHelper {
	once.Do(func() {
		addrHlp = &AddressHelper{
			ipAddresses: make(map[uint32]net.Addr),
			listeners:   make([]chan map[uint32]net.Addr, 0),
		}
		go func() {
			for {
				addrHlp.gatherAddresses()
				time.Sleep(100 * time.Millisecond)
			}
		}()
	})
	return addrHlp
}

func (a *AddressHelper) Subscribe(c chan map[uint32]net.Addr) {
	a.listeners = append(a.listeners, c)
}

func (a *AddressHelper) publish(msg map[uint32]net.Addr) {
	if len(a.listeners) > 0 && a.isInitalised {
		for _, c := range a.listeners {
			select {
			case c <- msg:
				godbg.Dbg(msg)
			default:
				godbg.Dbg("No accepting channels")
			}
		}
	}
}

func (a *AddressHelper) gatherAddresses() {
	a.ipAddresses = make(map[uint32]net.Addr)
	interfaces, _ := net.Interfaces()
	for _, iface := range interfaces {
		flags := iface.Flags.String()
		if !strings.Contains(flags, "loopback") {
			addrs, _ := iface.Addrs()
			for _, addr := range addrs {
				if !isLinkLocal(addr.String()) {
					arr := strings.Split(addr.String(), "/")
					if strings.Contains(arr[0], ":") {
						arr[0] = "[" + arr[0] + "]"
					}
					udpAddr, err := net.ResolveUDPAddr("udp", arr[0]+":4433")
					if err != nil {
						log.Println(err)
					} else {
						a.write(udpAddr)
					}

				}
			}
		}
	}
	a.isInitalised = true
	a.publish(a.ipAddresses)
}


func (a *AddressHelper) write(addr net.Addr) {
	checksum := crc32.ChecksumIEEE([]byte(addr.String()))
	a.ipAddresses[checksum] = addr
}

func (a *AddressHelper) containsAddress(addr net.Addr) bool {
	checksum := crc32.ChecksumIEEE([]byte(addr.String()))
	_, contains := a.ipAddresses[checksum]
	return contains
}

func isLinkLocal(addr string) bool {
	three := addr[0:3]
	four := addr[0:4]
	seven := addr[0:7]
	if three == "127" {
		return true
	} else if four == "fe80" {
		return true
	} else if seven == "169.254" {
		return true
	} else {
		return false
	}
}

func CRC(addr net.Addr) uint32 {
	return crc32.ChecksumIEEE([]byte(addr.String()))
}
