package quic

import (
	"errors"
	"fmt"
	"github.com/boisjacques/golang-utils"
	"github.com/boisjacques/qed/internal/wire"
	"github.com/sasha-s/go-deadlock"
	"github.com/tylerwince/godbg"
	"hash/crc32"
	"net"
)

type SchedulerRoundRobin struct {
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
	deletionQueue   []net.Addr
	additionQueue   []net.Addr
	lockRemote      deadlock.RWMutex
	lockLocal       deadlock.RWMutex
	lockPaths       deadlock.RWMutex
	lockAQ          deadlock.RWMutex
	lockDQ          deadlock.RWMutex
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
		addrChan:        make(chan map[uint32]net.Addr, 1000),
		localAddrs:      make(map[uint32]net.Addr),
		remoteAddrs:     make(map[uint32]net.Addr),
		sockets:         make(map[uint32]net.PacketConn),
		deletionQueue:   make([]net.Addr, 0),
		additionQueue:   make([]net.Addr, 0),
		lockRemote:      deadlock.RWMutex{},
		lockLocal:       deadlock.RWMutex{},
		lockPaths:       deadlock.RWMutex{},
		lockAQ:          deadlock.RWMutex{},
		lockDQ:          deadlock.RWMutex{},
		isInitialized:   false,
		totalPathWeight: 1000,
		isActive:        false,
	}
	scheduler.listenOnChannel()
	godbg.Dbg("Scheduler up and running")
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
	usock, err := s.openSocket(local)
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
	for _, remote := range s.remoteAddrs {
		if isSameVersion(local, remote) {
			s.newPath(local, remote)
		}
	}
}

func (s *SchedulerRoundRobin) addRemoteAddress(addr net.Addr) {
	checksum := CRC(addr)
	if !s.containsBlocking(checksum, remote) {
		s.remoteAddrs[checksum] = addr
		s.lockLocal.RLock()
		defer s.lockLocal.RUnlock()
		for _, laddr := range s.localAddrs {
			if isSameVersion(laddr, addr) {
				s.newPath(laddr, addr)
			}
		}
	}
}

func (s *SchedulerRoundRobin) removeAddress(address net.Addr) {
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

func (s *SchedulerRoundRobin) GetPathZero() *Path {
	return s.pathZero
}

func (s *SchedulerRoundRobin) removePath(pathId uint32) {
	delete(s.paths, pathId)
}

func (s *SchedulerRoundRobin) listenOnChannel() {
	s.addressHelper.Subscribe(s.addrChan)
	godbg.Dbg("Subscribed to channel")
	go s.addressSubscriber()
	godbg.Dbg("Started address subscriber")
	go s.queueHandler()
	godbg.Dbg("Started queue handler")
}

func (s *SchedulerRoundRobin) addressSubscriber() {
	i := 0
	for !s.addressHelper.isInitalised {
		i++
		if i%1000 == 0 {
			godbg.Dbg("Waiting for session establishment")
		}
	}
	godbg.Dbg("Session established")
	for {
		select {
		case addrs := <-s.addrChan:
			for key, addr := range addrs {
				if !s.containsBlocking(key, local) {
					s.lockAQ.Lock()
					s.additionQueue = append(s.additionQueue, addr)
					s.lockAQ.Unlock()
				}
			}
			s.lockLocal.RLock()
			for key, addr := range s.localAddrs {
				_, contains := addrs[key]
				if !contains {
					s.lockDQ.Lock()
					s.deletionQueue = append(s.deletionQueue, addr)
					s.lockDQ.Unlock()
				}
			}
			s.lockLocal.RUnlock()
			s.lockLocal.Lock()
			s.localAddrs = make(map[uint32]net.Addr)
			for key, value := range addrs {
				s.localAddrs[key] = value
			}
			s.lockLocal.Unlock()
		default:

		}
	}
}

func (s *SchedulerRoundRobin) queueHandler() {
	for {
		if s.isActive && s.session != nil {
			s.processAdditionQueue()
			s.processDeletionQueue()
		}
	}
}

func (s *SchedulerRoundRobin) processAdditionQueue() {
	s.lockAQ.Lock()
	defer s.lockAQ.Unlock()
	if len(s.additionQueue) > 0 {
		addr := s.additionQueue[0]
		s.addLocalAddress(addr)
		s.session.(*session).queueControlFrame(s.assembleAddrModFrame(wire.AddFrame, addr))
		s.session.(*session).logger.Debugf("Queued addition frame for address %s", addr.String())
		s.additionQueue[0] = nil
		s.additionQueue = s.additionQueue[1:]
	}
}

func (s *SchedulerRoundRobin) processDeletionQueue() {
	s.lockDQ.Lock()
	defer s.lockDQ.Unlock()
	if len(s.deletionQueue) > 0 {
		addr := s.deletionQueue[0]
		s.session.(*session).queueControlFrame(s.assembleAddrModFrame(wire.DeleteFrame, addr))
		s.session.(*session).logger.Debugf("Queued deletion frame for address %s", addr.String())
		for pathID, path := range s.paths {
			if path.contains(addr) {
				s.removePath(pathID)
			}
		}
		s.deletionQueue[0] = nil
		s.deletionQueue = s.deletionQueue[1:]
	}
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

func (s *SchedulerRoundRobin) containsBlocking(key uint32, direcion direcionAddr) bool {
	var contains bool
	if direcion == local {
		s.lockLocal.Lock()
		_, contains = s.localAddrs[key]
		s.lockLocal.Unlock()
	} else if direcion == remote {
		s.lockRemote.Lock()
		_, contains = s.remoteAddrs[key]
		s.lockRemote.Unlock()
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
		s.lockLocal.Lock()
		delete(s.localAddrs, CRC(addr))
		s.lockLocal.Unlock()
	}
	if direction == remote {
		s.lockRemote.Lock()
		delete(s.remoteAddrs, CRC(addr))
		s.lockRemote.Unlock()
	}
}

func (s *SchedulerRoundRobin) deletePath(pathId uint32) {
	s.lockPaths.Lock()
	defer s.lockPaths.Unlock()
	delete(s.paths, pathId)
}

func (s *SchedulerRoundRobin) setOwd(id uint32, owd int64) error {
	return errors.New("cannot set OWD in non OWD scheduler")
}

func (s *SchedulerRoundRobin) openSocket(local net.Addr) (net.PacketConn, error) {
	var err error = nil
	usock, contains := s.sockets[CRC(local)]
	if !contains {
		usock, err = net.ListenUDP("udp", local.(*net.UDPAddr))
		s.sockets[CRC(local)] = usock
	}
	return usock, err
}
