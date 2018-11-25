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

type direcionAddr uint8

const (
	local  = direcionAddr(0)
	remote = direcionAddr(1)
)

type Scheduler struct {
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

func NewScheduler(session Session, pconn net.PacketConn, remote net.Addr) *Scheduler {
	pathZero := &Path{
		isPathZero: true,
		pathID:     0,
		weight:     1000,
		conn:       NewConn(pconn, remote),
		owd:        0,
	}
	paths := make(map[uint32]*Path)
	paths[pathZero.pathID] = pathZero
	pathIds := make([]uint32, 0)
	pathIds = append(pathIds, pathZero.pathID)
	return &Scheduler{
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

func (s *Scheduler) IsInitialized() bool {
	return s.isInitialized
}

func (s *Scheduler) SetIsInitialized(isInitialized bool) {
	s.isInitialized = isInitialized
}

func (s *Scheduler) IsActive() bool {
	return s.isActive
}

func (s *Scheduler) Activate(isActive bool) {
	s.isActive = isActive
}

func (s *Scheduler) Send(p []byte) error {
	s.lockPaths.RLock()
	defer s.lockPaths.RUnlock()
	path, err := s.weightedSelect()
	if err != nil {
		fmt.Println(err, util.Tracer())
		return err
	}
	err = path.conn.Write(p)
	if err != nil {
		fmt.Println(err, util.Tracer())
		return err
	}
	if path.pathID == 0 {
	} else {
	}
	return nil
}

func (s *Scheduler) roundRobin() *Path {
	s.lastPath = (s.lastPath + 1) % uint32(len(s.pathIds))
	return s.paths[s.pathIds[s.lastPath]]
}

func (s *Scheduler) sendToPath(pathID uint32, p []byte) error {
	err := s.paths[pathID].conn.Write(p)
	if err != nil {
		fmt.Println(err, util.Tracer())
		return err
	}
	return nil
}

// TODO: Implement proper source address handling
func (s *Scheduler) newPath(local, remote net.Addr) {
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
	//p.updateMetric(s.referenceRTT)
	s.paths[p.pathID] = p
	s.pathIds = append(s.pathIds, p.pathID)
	s.sumUpWeights()
}

func (s *Scheduler) addLocalAddress(local net.Addr) {
	for remote := range s.remoteAddrs {
		if isSameVersion(local, remote) {
			s.newPath(local, remote)
		}
	}
}

func (s *Scheduler) addRemoteAddress(addr net.Addr) {
	if !s.containsBlocking(addr, remote) {
		s.remoteAddrs[addr] = struct{}{}
		s.addressHelper.lockAddresses.RLock()
		defer s.addressHelper.lockAddresses.RUnlock()
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

func (s *Scheduler) removeAddress(address net.Addr) {
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

func (s *Scheduler) initializePaths() {
	s.addressHelper.lockAddresses.RLock()
	s.lockRemote.RLock()
	defer s.addressHelper.lockAddresses.RUnlock()
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

func (s *Scheduler) GetPathZero() *Path {
	return s.pathZero
}

func (s *Scheduler) removePath(pathId uint32) {
	delete(s.paths, pathId)
}

func (s *Scheduler) ListenOnChannel() {
	s.addressHelper.Subscribe(s.addrChan)
	go func() {
		oldTime := time.Now().Second()
		for {
			if s.isActive {
				addr := <-s.addrChan
				if !s.containsBlocking(addr, local) {
					s.write(addr)
					s.session.(*session).QueueQedFrame(s.assembleAddrModFrame(wire.AddFrame, addr))
					s.session.(*session).logger.Debugf("Queued addition frame for address %s", addr.String())
				} else {
					s.delete(addr, local)
					s.session.(*session).QueueQedFrame(s.assembleAddrModFrame(wire.DeleteFrame, addr))
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

func (s *Scheduler) assembleAddrModFrame(operation wire.AddressModificationOperation, addr net.Addr) *wire.AddrModFrame {
	var version wire.IpVersion
	if addr.(*net.UDPAddr).IP.To4() != nil {
		version = wire.IPv4
	} else {
		version = wire.IPv6
	}
	f := wire.NewAddrModFrame(operation, version, addr)
	return f
}

func (s *Scheduler) assembleOwdFrame(pathId uint32) *wire.OwdFrame {
	f := wire.NewOwdFrame(pathId)
	return f
}

func xor(local, remote []byte) []byte {
	rval := make([]byte, 0)
	for i := 0; i < len(local); i++ {
		rval = append(rval, local[i])
	}
	for i := 0; i < len(remote); i++ {
		rval = append(rval, remote[i])
	}

	return rval
}

func isSameVersion(local, remote net.Addr) bool {
	if local.(*net.UDPAddr).IP.To4() == nil && remote.(*net.UDPAddr).IP.To4() == nil {
		return true
	}

	if local.(*net.UDPAddr).IP.To4() != nil && remote.(*net.UDPAddr).IP.To4() != nil {
		return true
	}
	return false
}

func (s *Scheduler) containsBlocking(addr net.Addr, direcion direcionAddr) bool {
	var contains bool
	if direcion == local {
		s.addressHelper.lockAddresses.RLock()
		defer s.addressHelper.lockAddresses.RUnlock()
		_, contains = s.localAddrs[addr]
	} else if direcion == remote {
		s.lockRemote.Lock()
		defer s.lockRemote.Unlock()
		_, contains = s.remoteAddrs[addr]
	}
	return contains
}

func (s *Scheduler) delete(addr net.Addr, direction direcionAddr) {
	for key, path := range s.paths {
		if path.contains(addr) {
			s.deletePath(key)
		}
	}
	if direction == local {
		s.addressHelper.lockAddresses.Lock()
		defer s.addressHelper.lockAddresses.Unlock()
		delete(s.localAddrs, addr)
	}
	if direction == remote {
		s.lockRemote.Lock()
		defer s.lockRemote.Unlock()
		delete(s.remoteAddrs, addr)
	}
}

func (s *Scheduler) deletePath(pathId uint32) {
	s.lockPaths.Lock()
	defer s.lockPaths.Unlock()
	delete(s.paths, pathId)
}

func (s *Scheduler) write(addr net.Addr) {
	s.addressHelper.lockAddresses.Lock()
	defer s.addressHelper.lockAddresses.Unlock()
	s.localAddrs[addr] = false
}

func (s *Scheduler) setOwd(pathID uint32, owd int64) {
	s.lockPaths.Lock()
	defer s.lockPaths.Unlock()
	s.paths[pathID].setOwd(owd)
}

func (s *Scheduler) measurePathsRunner() {
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
func (s *Scheduler) measurePaths() {
	for _, path := range s.paths {
		s.measurePath(path)
	}
}

//TODO: Time has to move further down the path
func (s *Scheduler) measurePath(path *Path) {
	s.lockPaths.RLock()
	defer s.lockPaths.RUnlock()
	s.session.(*session).QueueQedFrame(s.assembleOwdFrame(path.pathID))
}

func (s *Scheduler) sumUpWeights() {
	s.lockPaths.RLock()
	defer s.lockPaths.RUnlock()
	s.totalPathWeight = 0
	for _, path := range s.paths {
		s.totalPathWeight += path.weight
	}
}

func (s *Scheduler) weightedSelect() (*Path, error) {
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

func (s *Scheduler) weighPathsRunner() {
	go func() {
		for {
			if s.isActive {
				s.weighPaths()
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()
}

func (s *Scheduler) weighPaths() {
	s.lockPaths.Lock()
	for _, path := range s.paths {
		s.weighPath(path)
	}
	s.sumUpWeights()
}

func (s *Scheduler) weighPath(path *Path) {
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
func (s *Scheduler) calculateAverageOwd() uint64 {
	var aOwd uint64
	for _, path := range s.paths {
		aOwd += path.owd
	}
	aOwd = aOwd / uint64(len(s.paths))
	return aOwd
}

func (s *Scheduler) announceAddresses() {
	go func() {
		for !s.isActive {

		}
		for addr := range s.localAddrs {
			s.session.(*session).QueueQedFrame(s.assembleAddrModFrame(wire.AddFrame, addr))
			s.session.(*session).logger.Debugf("Queued addition frame for address %s", addr.String())
		}
	}()
}
