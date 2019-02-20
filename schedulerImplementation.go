package quic

import (
	"errors"
	"fmt"
	"github.com/boisjacques/golang-utils"
	"github.com/boisjacques/qed/internal/wire"
	"github.com/tylerwince/godbg"
	"hash/crc32"
	"math/rand"
	"net"
	"time"
)

type SchedulerImplementation struct {
	paths           map[uint32]*Path
	session         Session
	referenceRTT    uint16
	pathZero        *Path
	pathIds         []uint32
	lastPath        uint32
	addressHelper   *AddressHelper
	addrChan        chan map[uint32]net.Addr
	localAddrs      map[uint32]net.Addr
	remoteAddrs     map[uint32]net.Addr
	sockets         map[uint32]net.PacketConn
	isInitialized   bool
	totalPathWeight int
	isActive        bool
	mode            schedulerOperation
}

func NewScheduler(session Session, pconn net.PacketConn, remote net.Addr) *SchedulerImplementation {
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
	scheduler := &SchedulerImplementation{
		paths:           paths,
		session:         session,
		referenceRTT:    0,
		pathZero:        pathZero,
		pathIds:         pathIds,
		lastPath:        0,
		addressHelper:   NewAddressHelper(),
		addrChan:        make(chan map[uint32]net.Addr, 1000),
		localAddrs:      make(map[uint32]net.Addr),
		remoteAddrs:     make(map[uint32]net.Addr),
		sockets:         make(map[uint32]net.PacketConn),
		isInitialized:   false,
		totalPathWeight: 1000,
		isActive:        false,
		mode:            roundRobin,
	}
	scheduler.sockets[CRC(pconn.LocalAddr())] = pconn
	for !scheduler.addressHelper.isInitalised {

	}
	scheduler.localAddrs = scheduler.addressHelper.GetAddresses()
	go scheduler.announceAddresses()
	godbg.Dbg("Scheduler up and running")
	godbg.Dbg(scheduler.sockets)
	return scheduler
}

func (s *SchedulerImplementation) IsInitialized() bool {
	return s.isInitialized
}

func (s *SchedulerImplementation) SetIsInitialized(isInitialized bool) {
	s.isInitialized = isInitialized
}

func (s *SchedulerImplementation) IsActive() bool {
	return s.isActive
}

func (s *SchedulerImplementation) Activate(isActive bool) {
	s.isActive = isActive
}

func (s *SchedulerImplementation) Write(p []byte) error {
	var path *Path
	var err error
	for {
		path, err = s.getPath()
		if err != nil {

		}
		godbg.Dbg(path.Write())
		if path != nil && path.local != nil {
			godbg.Dbg(path.Write())
			break
		}
		s.session.(*session).logger.Errorf("nil path selected")
	}
	godbg.Dbg(path.Write())
	_, err = path.local.WriteTo(p, path.remote)
	if err != nil {
		fmt.Println(err, util.Tracer())
		return err
	}
	return nil
}

func (s *SchedulerImplementation) Read([]byte) (int, net.Addr, error) { return 0, nil, errors.New("Not implemented yet") }
func (s *SchedulerImplementation) Close() error {
	// TODO: Mock close
	return errors.New("not implemented yet")
}
func (s *SchedulerImplementation) LocalAddr() net.Addr           { return nil }
func (s *SchedulerImplementation) RemoteAddr() net.Addr          { return s.paths[s.lastPath].remote }
func (s *SchedulerImplementation) SetCurrentRemoteAddr(net.Addr) {}

func (s *SchedulerImplementation) getPath() (*Path, error) {
	if s.mode == roundRobin {
		return s.getRoundRobinPath()
	} else if s.mode == weightBased {
		return s.getWeightedPath()
	} else {
		return nil, errors.New("invalid mode of scheduler operation")
	}
}

func (s *SchedulerImplementation) getWeightedPath() (*Path, error) {
	rand.Seed(time.Now().UnixNano())
	r := rand.Intn(s.totalPathWeight)
	for _, path := range s.paths {
		r -= path.weight
		if r <= 0 {
			return path, nil
		}
	}
	return nil, errors.New("no path selected")
}

func (s *SchedulerImplementation) getRoundRobinPath() (*Path, error) {
	s.lastPath = (s.lastPath + 1) % uint32(len(s.pathIds))
	path := s.paths[s.pathIds[s.lastPath]]
	return path, nil
}

func (s *SchedulerImplementation) newPath(local, remote net.Addr) error {
	usock, err := s.openSocket(local)
	if err != nil {
		godbg.Dbg(err)
		return err
	}
	if usock == nil {
		err := errors.New("no socket returned")
		godbg.Dbg(err)
		return err
	}
	checksum := crc32.ChecksumIEEE(xor([]byte(local.String()), []byte(remote.String())))
	godbg.Dbg(checksum)
	p := NewPath(checksum, usock, remote, 1000)
	s.paths[p.pathID] = p
	s.pathIds = append(s.pathIds, p.pathID)
	return nil
}

func (s *SchedulerImplementation) addLocalAddress(local net.Addr) {
	for _, remote := range s.remoteAddrs {
		if isSameVersion(local, remote) {
			err := s.newPath(local, remote)
			if err != nil {
				s.session.(*session).logger.Errorf("Path could not be created: %s", err)
			}
		}
	}
}

func (s *SchedulerImplementation) addRemoteAddress(remoteAddress net.Addr) {
	checksum := CRC(remoteAddress)
	if !s.containsBlocking(checksum, remote) {
		s.remoteAddrs[checksum] = remoteAddress
		for _, localAddress := range s.localAddrs {
			if isSameVersion(localAddress, remoteAddress) {
				err := s.newPath(localAddress, remoteAddress)
				if err != nil {
					s.session.(*session).logger.Errorf("Path could not be created: %s", err)
				}
			}
		}
	}
}

func (s *SchedulerImplementation) removeAddress(address net.Addr) {
	if s.containsBlocking(CRC(address), remote) {
		s.delete(address, remote)
	}
	if s.containsBlocking(CRC(address), local) {
		s.delete(address, local)
	}
	for k, v := range s.paths {
		if v.contains(address) {
			s.removePath(k)
		}
	}
}

func (s *SchedulerImplementation) GetPathZero() *Path {
	return s.pathZero
}

func (s *SchedulerImplementation) removePath(pathId uint32) {
	delete(s.paths, pathId)
}

func (s *SchedulerImplementation) announceAddresses() {
	sessCtr := 0
	actCtr := 0
	for s.session == nil {
		sessCtr++
		if sessCtr%100000 == 0 {
			godbg.Dbg("Nilsession")
		}
	}
	for !s.isActive {
		actCtr++
		if actCtr%100000 == 0 {
			godbg.Dbg("Scheduler inactive")
		}
	}
	for _, addr := range s.localAddrs {
		if addr != s.pathZero.local.LocalAddr() {
			s.session.(*session).queueControlFrame(s.assembleAddrModFrame(wire.AddFrame, addr))
			s.session.(*session).logger.Debugf("Queued addition frame for address %s", addr.String())
		}
	}
}

func (s *SchedulerImplementation) assembleAddrModFrame(operation wire.AddressModificationOperation, addr net.Addr) *wire.AddrModFrame {
	var version wire.IpVersion
	if addr.(*net.UDPAddr).IP.To4() != nil {
		version = wire.IPv4
	} else {
		version = wire.IPv6
	}
	f := wire.NewAddrModFrame(operation, version, addr)
	return f
}

func (s *SchedulerImplementation) assembleOwdFrame(pathId uint32) *wire.OwdFrame {
	f := wire.NewOwdFrame(pathId)
	return f
}

func (s *SchedulerImplementation) containsBlocking(key uint32, direcion direcionAddr) bool {
	var contains bool
	if direcion == local {
		_, contains = s.localAddrs[key]
	} else if direcion == remote {
		_, contains = s.remoteAddrs[key]
	}
	return contains
}

func (s *SchedulerImplementation) delete(addr net.Addr, direction direcionAddr) {
	for key, path := range s.paths {
		if path.contains(addr) {
			s.deletePath(key)
		}
	}
	if direction == local {
		delete(s.localAddrs, CRC(addr))
	}
	if direction == remote {
		delete(s.remoteAddrs, CRC(addr))
	}
}

func (s *SchedulerImplementation) deletePath(pathId uint32) {
	delete(s.paths, pathId)
}

func (s *SchedulerImplementation) setOwd(pathId uint32, owd int64) error {
	s.paths[pathId].setOwd(owd)
	return nil
}

func (s *SchedulerImplementation) openSocket(local net.Addr) (net.PacketConn, error) {
	var err error = nil
	usock, contains := s.sockets[CRC(local)]
	if !contains {
		usock, err = net.ListenUDP("udp", local.(*net.UDPAddr))
		if usock != nil {
			s.sockets[CRC(local)] = usock
		}
	}
	return usock, err
}

func (s *SchedulerImplementation) measurePathsRunner() {
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
func (s *SchedulerImplementation) measurePaths() {
	for _, path := range s.paths {
		s.measurePath(path)
	}
}

//TODO: Time has to move further down the path
func (s *SchedulerImplementation) measurePath(path *Path) {
	s.session.(*session).queueControlFrame(s.assembleOwdFrame(path.pathID))
}

func (s *SchedulerImplementation) sumUpWeights() {
	s.totalPathWeight = 0
	for _, path := range s.paths {
		s.totalPathWeight += path.weight
	}
}

func (s *SchedulerImplementation) weighPathsRunner() {
	go func() {
		for {
			if s.isActive {
				s.weighPaths()
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()
}

func (s *SchedulerImplementation) weighPaths() {
	for _, path := range s.paths {
		s.weighPath(path)
	}
	s.sumUpWeights()
}

func (s *SchedulerImplementation) weighPath(path *Path) {

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
func (s *SchedulerImplementation) calculateAverageOwd() uint64 {
	var aOwd uint64
	for _, path := range s.paths {
		aOwd += path.owd
	}
	aOwd = aOwd / uint64(len(s.paths))
	return aOwd
}
