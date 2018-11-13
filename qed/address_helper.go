package quic

import (
	"github.com/boisjacques/golang-utils"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

type AddressHelper struct {
	ipAddresses       map[string]*net.UDPAddr
	ipAddressesBool   map[string]bool
	sockets           map[string]*net.UDPConn
	listeners         []chan string
	lockAddresses     sync.RWMutex
	lockAddressesBool sync.RWMutex
	lockSockets       sync.RWMutex
}

func NewAddressHelper() *AddressHelper {
	ah := AddressHelper{
		make(map[string]*net.UDPAddr),
		make(map[string]bool),
		make(map[string]*net.UDPConn),
		make([]chan string, 0),
		sync.RWMutex{},
		sync.RWMutex{},
		sync.RWMutex{},
	}
	ah.GatherAddresses()
	return &ah
}

func (a *AddressHelper) Subscribe(c chan string) {
	a.listeners = append(a.listeners, c)
}

func (a *AddressHelper) Publish(msg string) {
	if len(a.listeners) > 0 {
		for _, c := range a.listeners {
			c <- msg
		}
	}
}

func (a *AddressHelper) GatherAddresses() {
	a.falsifyAddresses()
	interfaces, _ := net.Interfaces()
	for _, iface := range interfaces {
		flags := iface.Flags.String()
		if !strings.Contains(flags, "loopback") {
			addrs, _ := iface.Addrs()
			for _, addr := range addrs {
				//TODO: Fix link local addresses, add 169.254.0.0/16
				if !isLinkLocal(addr.String()) {
					if strings.Contains(addr.String(), ":") {

					}
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
							a.Publish(udpAddr.String())
							}
						}
					}
				}
			}
		}
	a.cleanUp()
}

func (a *AddressHelper) openSocket(local *net.UDPAddr) (*net.UDPConn, error) {
	a.lockSockets.Lock()
	log.Println("locked", util.Tracer())
	defer a.lockSockets.Unlock()
	defer log.Println("unlocked", util.Tracer())
	var err error = nil
	usock, contains := a.sockets[local.String()]
	if !contains {
		usock, err = net.ListenUDP("udp", local)
		a.sockets[local.String()] = usock
	}
	return usock, err
}

func (a *AddressHelper) cleanUp() {
	a.lockAddresses.Lock()
	a.lockAddressesBool.Lock()
	log.Println("locked: ", util.Tracer())
	defer a.lockAddresses.Unlock()
	defer a.lockAddressesBool.Unlock()
	defer log.Println("unlocked: ", util.Tracer())
	for key, value := range a.ipAddressesBool {
		if value == false {
			a.Publish(key)
			time.Sleep(100 * time.Millisecond) //Wait 100 ms for handling in scheduler
			delete(a.ipAddresses, key)
			a.sockets[key].Close()
		}
	}
}

func (a *AddressHelper) GetAddresses() *map[string]*net.UDPAddr {
	a.lockAddresses.RLock()
	log.Println("locked: ", util.Tracer())
	defer a.lockAddresses.RUnlock()
	defer log.Println("unlocked: ", util.Tracer())
	return &a.ipAddresses
}

func (a *AddressHelper) write(addr *net.UDPAddr, bool bool) {
	a.lockAddresses.Lock()
	a.lockAddressesBool.Lock()
	log.Println("locked: ", util.Tracer())
	defer a.lockAddressesBool.Unlock()
	defer a.lockAddresses.Unlock()
	defer log.Println("unlocked: ", util.Tracer())
	a.ipAddresses[addr.String()] = addr
	a.ipAddressesBool[addr.String()] = bool
}

func (a *AddressHelper) containsAddress(addr *net.UDPAddr) bool {
	a.lockAddresses.RLock()
	log.Println("locked: ", util.Tracer())
	defer a.lockAddresses.RUnlock()
	defer log.Println("unlocked: ", util.Tracer())
	_, contains := a.ipAddresses[addr.String()]
	return contains
}

func (a *AddressHelper) containsSocket(addr *net.UDPAddr) bool {
	a.lockSockets.RLock()
	log.Println("locked: ", util.Tracer())
	defer a.lockSockets.RUnlock()
	defer log.Println("unlocked: ", util.Tracer())
	_, contains := a.sockets[addr.String()]
	return contains
}

func (a *AddressHelper) falsifyAddresses() {
	a.lockAddresses.Lock()
	a.lockAddressesBool.Lock()
	log.Println("locked: ", util.Tracer())
	defer a.lockAddresses.Unlock()
	defer a.lockAddressesBool.Unlock()
	defer log.Println("unlocked: ", util.Tracer())
	for address := range a.ipAddresses {
		a.ipAddressesBool[address] = false
	}
}

func isLinkLocal(addr string) bool {
	three := addr[0:3]
	four :=addr[0:4]
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