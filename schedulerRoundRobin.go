package quic

import (
	"errors"
	"fmt"
	"github.com/boisjacques/golang-utils"
	"github.com/boisjacques/qed/internal/wire"
	"hash/crc32"
	"log"
	"net"
	"sync"
	"time"
)

type SchedulerRoundRobin struct {
	paths           map[uint32]*Path
	session         Session
	referenceRTT    uint16
	pathZero        *Path
	pathIds         []uint32
	lastPath        uint32
	addressHelper   *AddressHelper
	addrChan        chan net.Addr
	localAddrs      map[net.Addr]bool
	remoteAddrs     map[net.Addr]struct{}
	lockRemote      sync.RWMutex
	lockPaths       sync.RWMutex
	isInitialized   bool
	totalPathWeight int
	isActive        bool
}

func NewSchedulerRoundRobin(session Session, pconn net.PacketConn, remote net.Addr) *SchedulerRoundRobin {
	pathZero := &Path{
		isPathZero: true,
		pathID:     0,
		weight:     1000,
		local:      pconn,
		remote:     remote,
		owd:        0,
	}
	paths := make(map[uint32]*Path)
	paths[pathZero.pathID] = pathZero
	pathIds := make([]uint32, 0)
	pathIds = append(pathIds, pathZero.pathID)
	scheduler := &SchedulerRoundRobin{
		paths:           paths,
		session:         session,
		referenceRTT:    0,
		pathZero:        pathZero,
		pathIds:         pathIds,
		lastPath:        0,
		addressHelper:   GetAddressHelper(),
		addrChan:        make(chan net.Addr),
		localAddrs:      GetAddressHelper().ipAddresses,
		remoteAddrs:     make(map[net.Addr]struct{}),
		lockRemote:      sync.RWMutex{},
		lockPaths:       sync.RWMutex{},
		isInitialized:   false,
		totalPathWeight: 1000,
		isActive:        false,
	}
	scheduler.listenOnChannel()
	return scheduler
}

func (s *SchedulerRoundRobin) IsInitialized() bool {
	return s.isInitialized
}

func (s *SchedulerRoundRobin) SetIsInitialized(isInitialized bool) {
	s.isInitialized = isInitialized
}

func (s *SchedulerRoundRobin) IsActive() bool {
	return s.isActive
}

func (s *SchedulerRoundRobin) Activate(isActive bool) {
	s.isActive = isActive
}

func (s *SchedulerRoundRobin) Write(p []byte) error {
	s.lockPaths.RLock()
	defer s.lockPaths.RUnlock()
	path := s.roundRobin()

	_, err := path.local.WriteTo(p, path.remote)
	if err != nil {
		fmt.Println(err, util.Tracer())
		return err
	}
	if path.pathID == 0 {
	} else {
	}
	return nil
}

func (s *SchedulerRoundRobin) Read([]byte) (int, net.Addr, error) { return 0, nil, errors.New("Not implemented yet") }
func (s *SchedulerRoundRobin) Close() error {
	// TODO: Mock close
	return errors.New("Not implemented yet")
}
func (s *SchedulerRoundRobin) LocalAddr() net.Addr           { return nil }
func (s *SchedulerRoundRobin) RemoteAddr() net.Addr          { return s.paths[s.lastPath].remote }
func (s *SchedulerRoundRobin) SetCurrentRemoteAddr(net.Addr) {}

func (s *SchedulerRoundRobin) roundRobin() *Path {
	s.lastPath = (s.lastPath + 1) % uint32(len(s.pathIds))
	return s.paths[s.pathIds[s.lastPath]]
}

// TODO: Implement proper source address handling
func (s *SchedulerRoundRobin) newPath(local, remote net.Addr) {
	usock, err := s.addressHelper.openSocket(local)
	if err != nil {
		s.session.(*session).logger.Errorf("Path could not be created because of %s", err)
		return
	}
	checksum := crc32.ChecksumIEEE(xor([]byte(local.String()), []byte(remote.String())))

	p := NewPath(checksum, usock, remote, 1000)
	s.paths[p.pathID] = p
	s.pathIds = append(s.pathIds, p.pathID)
}

func (s *SchedulerRoundRobin) addLocalAddress(local net.Addr) {
	for remote := range s.remoteAddrs {
		if isSameVersion(local, remote) {
			s.newPath(local, remote)
		}
	}
}

func (s *SchedulerRoundRobin) addRemoteAddress(addr net.Addr) {
	if !s.containsBlocking(addr, remote) {
		s.remoteAddrs[addr] = struct{}{}
		s.addressHelper.mutex.RLock()
		defer s.addressHelper.mutex.RUnlock()
		for laddr := range s.localAddrs {
			if isSameVersion(laddr, addr) {
				s.newPath(laddr, addr)
			}
		}
		remoteAdded := true
		if remoteAdded {
			//For breakpoints only
		}
	}
}

func (s *SchedulerRoundRobin) removeAddress(address net.Addr) {
	if s.containsBlocking(address, remote) {
		s.delete(address, remote)
	}
	if s.containsBlocking(address, local) {
		s.delete(address, local)
	}
	for k, v := range s.paths {
		if v.contains(address) {
			s.removePath(k)
		}
	}
}

func (s *SchedulerRoundRobin) initializePaths() {
	s.addressHelper.mutex.RLock()
	s.lockRemote.RLock()
	defer s.addressHelper.mutex.RUnlock()
	defer s.lockRemote.RUnlock()
	for local := range s.localAddrs {
		for remote := range s.remoteAddrs {
			if isSameVersion(local, remote) {
				s.newPath(local, remote)
			}
		}
	}
	s.isInitialized = true
}

func (s *SchedulerRoundRobin) GetPathZero() *Path {
	return s.pathZero
}

func (s *SchedulerRoundRobin) removePath(pathId uint32) {
	delete(s.paths, pathId)
}

func (s *SchedulerRoundRobin) listenOnChannel() {
	for !s.addressHelper.isInitalised {

	}
	s.localAddrs = s.addressHelper.ipAddresses
	s.addressHelper.Subscribe(s.addrChan)
	go func() {
		oldTime := time.Now().Second()
		for {
			if s.isActive && s.session != nil {

				addr := <-s.addrChan
				if !s.containsBlocking(addr, local) {
					s.write(addr)
					s.session.(*session).queueControlFrame(s.assembleAddrModFrame(wire.AddFrame, addr))
					s.session.(*session).logger.Debugf("Queued addition frame for address %s", addr.String())
				} else {
					s.delete(addr, local)
					s.session.(*session).queueControlFrame(s.assembleAddrModFrame(wire.DeleteFrame, addr))
					s.session.(*session).logger.Debugf("Queued deletion frame for address %s", addr.String())
				}
			} else if s.isActive && s.session == nil {
				log.Fatalf("Uexpected nil session at %s", util.Tracer())
				panic("Just panic")
			} else {
				if time.Now().Second()-oldTime == 10 {
					s.session.(*session).logger.Debugf("Waiting for connection establishment...")
					oldTime = time.Now().Second()
				}
			}
		}
	}()
}

func (s *SchedulerRoundRobin) assembleAddrModFrame(operation wire.AddressModificationOperation, addr net.Addr) *wire.AddrModFrame {
	var version wire.IpVersion
	if addr.(*net.UDPAddr).IP.To4() != nil {
		version = wire.IPv4
	} else {
		version = wire.IPv6
	}
	f := wire.NewAddrModFrame(operation, version, addr)
	return f
}

func (s *SchedulerRoundRobin) assembleOwdFrame(pathId uint32) *wire.OwdFrame {
	f := wire.NewOwdFrame(pathId)
	return f
}

func (s *SchedulerRoundRobin) containsBlocking(addr net.Addr, direcion direcionAddr) bool {
	var contains bool
	if direcion == local {
		s.addressHelper.mutex.RLock()
		defer s.addressHelper.mutex.RUnlock()
		_, contains = s.localAddrs[addr]
	} else if direcion == remote {
		s.lockRemote.Lock()
		defer s.lockRemote.Unlock()
		_, contains = s.remoteAddrs[addr]
	}
	return contains
}

func (s *SchedulerRoundRobin) delete(addr net.Addr, direction direcionAddr) {
	for key, path := range s.paths {
		if path.contains(addr) {
			s.deletePath(key)
		}
	}
	if direction == local {
		s.addressHelper.mutex.Lock()
		defer s.addressHelper.mutex.Unlock()
		delete(s.localAddrs, addr)
	}
	if direction == remote {
		s.lockRemote.Lock()
		defer s.lockRemote.Unlock()
		delete(s.remoteAddrs, addr)
	}
}

func (s *SchedulerRoundRobin) deletePath(pathId uint32) {
	s.lockPaths.Lock()
	defer s.lockPaths.Unlock()
	delete(s.paths, pathId)
}

func (s *SchedulerRoundRobin) write(addr net.Addr) {
	s.addressHelper.mutex.Lock()
	defer s.addressHelper.mutex.Unlock()
	s.localAddrs[addr] = false
}

func (s *SchedulerRoundRobin) setOwd(id uint32, owd int64) {
	// Dummy Method
}
