package quic

import (
	"fmt"
	"github.com/boisjacques/golang-utils"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

type AddressHelper struct {
	ipAddresses   map[net.Addr]bool
	sockets       map[net.Addr]net.PacketConn
	listeners     []chan net.Addr
	lockAddresses sync.RWMutex
	lockSockets   sync.RWMutex
}

var addrHlp *AddressHelper
var once sync.Once

func GetAddressHelper() *AddressHelper {
	once.Do(func() {
		addrHlp = &AddressHelper{
			make(map[net.Addr]bool),
			make(map[net.Addr]net.PacketConn),
			make([]chan net.Addr, 0),
			sync.RWMutex{},
			sync.RWMutex{},
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

func (a *AddressHelper) Subscribe(c chan net.Addr) {
	a.listeners = append(a.listeners, c)
}

func (a *AddressHelper) publish(msg net.Addr) {
	if len(a.listeners) > 0 {
		for _, c := range a.listeners {
			c <- msg
		}
	}
}

func (a *AddressHelper) gatherAddresses() {
	a.falsifyAddresses()
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
						if a.containsAddress(udpAddr) {
							a.write(udpAddr, true)
						}
						if !a.containsAddress(udpAddr) {
							a.write(udpAddr, true)
							a.publish(udpAddr)
						}
					}
				}
			}
		}
	}
	if err := a.cleanUp(); err != nil {
		log.Fatalf("error %s occurred during address handler clean up", err)
	}
}

func (a *AddressHelper) openSocket(local net.Addr) (net.PacketConn, error) {
	a.lockSockets.Lock()
	log.Printf("Locked Mutex %s", util.Tracer())
	defer a.lockSockets.Unlock()
	defer log.Printf("Unlocked Mutex %s", util.Tracer())
	var err error = nil
	usock, contains := a.sockets[local]
	if !contains {
		usock, err = net.ListenUDP("udp", local.(*net.UDPAddr))
		a.sockets[local] = usock
	}
	return usock, err
}

func (a *AddressHelper) cleanUp() error {
	a.lockAddresses.Lock()
	log.Printf("Locked Mutex %s", util.Tracer())
	defer a.lockAddresses.Unlock()
	defer log.Printf("Unlocked Mutex %s", util.Tracer())
	for key, value := range a.ipAddresses {
		if value == false {
			a.publish(key)
			time.Sleep(100 * time.Millisecond) //Wait 100 ms for handling in scheduler
			if a.containsSocket(key) {
				err := a.sockets[key].Close()
				if err != nil {
					fmt.Println(err)
					return err
				}
			}
			delete(a.ipAddresses, key)
		}
	}
	return nil
}

func (a *AddressHelper) GetAddresses() *map[net.Addr]bool {
	a.lockAddresses.RLock()
	log.Printf("Locked Mutex %s", util.Tracer())
	defer a.lockAddresses.RUnlock()
	defer log.Printf("Unlocked Mutex %s", util.Tracer())
	return &a.ipAddresses
}

func (a *AddressHelper) write(addr net.Addr, bool bool) {
	a.lockAddresses.Lock()
	log.Printf("Locked Mutex %s", util.Tracer())
	defer a.lockAddresses.Unlock()
	defer log.Printf("Unlocked Mutex %s", util.Tracer())
	a.ipAddresses[addr] = bool
}

func (a *AddressHelper) containsAddress(addr net.Addr) bool {
	a.lockAddresses.RLock()
	log.Printf("Locked Mutex %s", util.Tracer())
	defer a.lockAddresses.RUnlock()
	defer log.Printf("Unlocked Mutex %s", util.Tracer())
	_, contains := a.ipAddresses[addr]
	return contains
}

func (a *AddressHelper) containsSocket(addr net.Addr) bool {
	a.lockSockets.RLock()
	log.Printf("Locked Mutex %s", util.Tracer())
	defer a.lockSockets.RUnlock()
	defer log.Printf("Unlocked Mutex %s", util.Tracer())
	_, contains := a.sockets[addr]
	return contains
}

func (a *AddressHelper) falsifyAddresses() {
	a.lockAddresses.Lock()
	log.Printf("Locked Mutex %s", util.Tracer())
	defer a.lockAddresses.Unlock()
	defer log.Printf("Unlocked Mutex %s", util.Tracer())
	for address := range a.ipAddresses {
		a.ipAddresses[address] = false
	}
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
