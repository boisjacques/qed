package quic

import (
	"errors"
	"fmt"
	"github.com/boisjacques/golang-utils"
	"github.com/boisjacques/qed/internal/wire"
	"hash/crc32"
	"math/rand"
	"net"
	"sync"
	"time"
)

type SchedulerOwd struct {
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

func NewSchedulerOwd(session Session, pconn net.PacketConn, remote net.Addr) *SchedulerOwd {
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
	return &SchedulerOwd{
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
}

func (s *SchedulerOwd) IsInitialized() bool {
	return s.isInitialized
}

func (s *SchedulerOwd) SetIsInitialized(isInitialized bool) {
	s.isInitialized = isInitialized
}

func (s *SchedulerOwd) IsActive() bool {
	return s.isActive
}

func (s *SchedulerOwd) Activate(isActive bool) {
	s.isActive = isActive
}

func (s *SchedulerOwd) Write(p []byte) error {
	s.lockPaths.RLock()
	defer s.lockPaths.RUnlock()
	path, err := s.weightedSelect()
	if err != nil {
		fmt.Println(err, util.Tracer())
		return err
	}
	_, err = path.local.WriteTo(p, path.remote)
	if err != nil {
		fmt.Println(err, util.Tracer())
		return err
	}
	if path.pathID == 0 {
	} else {
	}
	return nil
}

func (s *SchedulerOwd) Read([]byte) (int, net.Addr, error) {return 0,nil, errors.New("Not implemented yet")}
func (s *SchedulerOwd) Close() error {return errors.New("Not implemented yet")}
func (s *SchedulerOwd) LocalAddr() net.Addr {return nil}
func (s *SchedulerOwd) RemoteAddr() net.Addr {return nil}
func (s *SchedulerOwd) SetCurrentRemoteAddr(net.Addr) {}

// TODO: Implement proper source address handling
func (s *SchedulerOwd) newPath(local, remote net.Addr) {
	usock, err := s.addressHelper.openSocket(local)
	if err != nil {
		s.session.(*session).logger.Errorf("Path could not be created because of %s", err)
		return
	}
	checksum := crc32.ChecksumIEEE(xor([]byte(local.String()), []byte(remote.String())))
	var weight int
	if len(s.paths) == 0 {
		weight = 1000
	} else {
		weight = 1000 / len(s.paths)
	}
	p := NewPath(checksum, usock, remote, weight)
	s.paths[p.pathID] = p
	s.pathIds = append(s.pathIds, p.pathID)
	s.sumUpWeights()
}

func (s *SchedulerOwd) addLocalAddress(local net.Addr) {
	for remote := range s.remoteAddrs {
		if isSameVersion(local, remote) {
			s.newPath(local, remote)
		}
	}
}

func (s *SchedulerOwd) addRemoteAddress(addr net.Addr) {
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

func (s *SchedulerOwd) removeAddress(address net.Addr) {
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

func (s *SchedulerOwd) initializePaths() {
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

func (s *SchedulerOwd) GetPathZero() *Path {
	return s.pathZero
}

func (s *SchedulerOwd) removePath(pathId uint32) {
	delete(s.paths, pathId)
}

func (s *SchedulerOwd) ListenOnChannel() {
	s.addressHelper.Subscribe(s.addrChan)
	go func() {
		oldTime := time.Now().Second()
		for {
			if s.isActive {
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
			} else {
				if time.Now().Second()-oldTime == 10 {
					s.session.(*session).logger.Debugf("Waiting for connection establishment...")
					oldTime = time.Now().Second()
				}
			}
		}
	}()
}

func (s *SchedulerOwd) assembleAddrModFrame(operation wire.AddressModificationOperation, addr net.Addr) *wire.AddrModFrame {
	var version wire.IpVersion
	if addr.(*net.UDPAddr).IP.To4() != nil {
		version = wire.IPv4
	} else {
		version = wire.IPv6
	}
	f := wire.NewAddrModFrame(operation, version, addr)
	return f
}

func (s *SchedulerOwd) assembleOwdFrame(pathId uint32) *wire.OwdFrame {
	f := wire.NewOwdFrame(pathId)
	return f
}


func (s *SchedulerOwd) containsBlocking(addr net.Addr, direcion direcionAddr) bool {
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

func (s *SchedulerOwd) delete(addr net.Addr, direction direcionAddr) {
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

func (s *SchedulerOwd) deletePath(pathId uint32) {
	s.lockPaths.Lock()
	defer s.lockPaths.Unlock()
	delete(s.paths, pathId)
}

func (s *SchedulerOwd) write(addr net.Addr) {
	s.addressHelper.mutex.Lock()
	defer s.addressHelper.mutex.Unlock()
	s.localAddrs[addr] = false
}

func (s *SchedulerOwd) setOwd(pathID uint32, owd int64) {
	s.lockPaths.Lock()
	defer s.lockPaths.Unlock()
	s.paths[pathID].setOwd(owd)
}

func (s *SchedulerOwd) measurePathsRunner() {
	go func() {
		for {
			if s.isActive {
				s.measurePaths()
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()
}

// If deadlock look here
func (s *SchedulerOwd) measurePaths() {
	for _, path := range s.paths {
		s.measurePath(path)
	}
}

//TODO: Time has to move further down the path
func (s *SchedulerOwd) measurePath(path *Path) {
	s.lockPaths.RLock()
	defer s.lockPaths.RUnlock()
	s.session.(*session).queueControlFrame(s.assembleOwdFrame(path.pathID))
}

func (s *SchedulerOwd) sumUpWeights() {
	s.lockPaths.RLock()
	defer s.lockPaths.RUnlock()
	s.totalPathWeight = 0
	for _, path := range s.paths {
		s.totalPathWeight += path.weight
	}
}

func (s *SchedulerOwd) weightedSelect() (*Path, error) {
	s.lockPaths.RLock()
	defer s.lockPaths.RUnlock()
	rand.Seed(time.Now().UnixNano())
	r := rand.Intn(s.totalPathWeight)
	for _, path := range s.paths {
		r -= path.weight
		if r <= 0 {
			return path, nil
		}
	}
	return nil, errors.New("No path selected")
}

func (s *SchedulerOwd) weighPathsRunner() {
	go func() {
		for {
			if s.isActive {
				s.weighPaths()
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()
}

func (s *SchedulerOwd) weighPaths() {
	s.lockPaths.Lock()
	for _, path := range s.paths {
		s.weighPath(path)
	}
	s.sumUpWeights()
}

func (s *SchedulerOwd) weighPath(path *Path) {
	s.lockPaths.Lock()
	defer s.lockPaths.Unlock()
	if path.owd < s.calculateAverageOwd() {
		if path.weight < 1000 {
			path.weight = path.weight + 1
		}
	} else {
		if path.weight > 1 {
			path.weight = path.weight - 1
		}
	}
}

// TODO: Put moving avg. here
func (s *SchedulerOwd) calculateAverageOwd() uint64 {
	var aOwd uint64
	for _, path := range s.paths {
		aOwd += path.owd
	}
	aOwd = aOwd / uint64(len(s.paths))
	return aOwd
}

func (s *SchedulerOwd) announceAddresses() {
	go func() {
		for !s.isActive {

		}
		for addr := range s.localAddrs {
			s.session.(*session).queueControlFrame(s.assembleAddrModFrame(wire.AddFrame, addr))
			s.session.(*session).logger.Debugf("Queued addition frame for address %s", addr.String())
		}
	}()
}
