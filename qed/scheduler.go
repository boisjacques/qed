package quic

import (
	"fmt"
	"github.com/boisjacques/golang-utils"
	"net"
	"sync"
	"time"
	"hash/crc32"
	"math/rand"
	"errors"
)

type direcionAddr uint8

const (
	local  = direcionAddr(0)
	remote = direcionAddr(1)
)

type Scheduler struct {
	paths          	map[uint32]*Path
	connection     	*Connection
	referenceRTT   	uint16
	pathZero       	*Path
	pathIds        	[]uint32
	lastPath       	uint32
	addressHelper  	*AddressHelper
	addrChan       	chan string
	localAddrs     	map[string]*net.UDPAddr
	localAddrsBool 	map[string]bool
	remoteAddrs    	map[string]*net.UDPAddr
	lockRemote     	sync.RWMutex
	lockPaths      	sync.RWMutex
	isInitialized  	bool
	totalPathWeight	int
}

func NewScheduler(initTrans Transport, connection *Connection, ah *AddressHelper) Scheduler {
	connection.log(logTypeMultipath, "New scheduler built for connection %v", connection.clientConnectionId)
	pathZero := &Path{
		connection,
		true,
		initTrans,
		0,
		1000,
		0,
		nil,
		nil,
	}
	paths := make(map[uint32]*Path)
	paths[pathZero.pathID] = pathZero
	pathIds := make([]uint32, 0)
	pathIds = append(pathIds, pathZero.pathID)
	return Scheduler{
		paths,
		connection,
		0,
		pathZero,
		pathIds,
		0,
		ah,
		make(chan string),
		ah.ipAddresses,
		ah.ipAddressesBool,
		make(map[string]*net.UDPAddr),
		sync.RWMutex{},
		sync.RWMutex{},
		false,
		1000,
	}
}

func (s *Scheduler) Send(p []byte) error {
	s.lockPaths.RLock()
	defer s.lockPaths.RUnlock()
	path,err := s.weightedSelect()
	if err != nil {
		fmt.Println(err, util.Tracer())
		return err
	}
	err = path.transport.Send(p)
	if err != nil {
		fmt.Println(err, util.Tracer())
		return err
	}
	if path.pathID == 0 {
		s.connection.log(logTypeMultipath, "Packet sent. Used path zero")
	} else {
		s.connection.log(logTypeMultipath, "Packet sent. local: %v \n remote: %x", s.paths[s.pathIds[int(s.lastPath)]].local, s.paths[s.pathIds[int(s.lastPath)]].remote)
	}
	return nil
}

func (s *Scheduler) roundRobin() *Path {
	s.lastPath = (s.lastPath + 1) % uint32(len(s.pathIds))
	return s.paths[s.pathIds[s.lastPath]]
}

func (s *Scheduler) sendToPath(pathID uint32, p []byte) error {
	err := s.paths[pathID].transport.Send(p)
	if err != nil {
		fmt.Println(err, util.Tracer())
		return err
	}
	return nil
}

// TODO: Implement proper source address handling
func (s *Scheduler) newPath(local, remote *net.UDPAddr) {
	usock, err := s.addressHelper.openSocket(local)
	if err != nil {
		s.connection.log(logTypeMultipath, "Error while creating path local IP: %x remote IP %v", *local, *remote)
		s.connection.log(logTypeMultipath, "Following error occurred", err)
		fmt.Println("Path could not be created")
		fmt.Println(err)
		return
	}
	transport := NewUdpTransport(usock, remote)
	checksum := crc32.ChecksumIEEE(xor([]byte(local.String()), []byte(remote.String())))
	var weight int
	if len(s.paths) == 0{
		weight = 1000
	} else {
		weight = 1000 / len(s.paths)
	}
	p := NewPath(s.connection, transport, checksum, local, remote, weight)
	s.connection.log(logTypeMultipath, "Path successfully created. Endpoints: local %v remote %x", local, remote)
	//p.updateMetric(s.referenceRTT)
	s.paths[p.pathID] = p
	s.pathIds = append(s.pathIds, p.pathID)
	s.sumUpWeights()
}

func (s *Scheduler) addLocalAddress(local net.UDPAddr) {
	s.connection.log(logTypeMultipath, "Adding local address %v", local)
	for _, remote := range s.remoteAddrs {
		if isSameVersion(&local, remote) {
			s.newPath(&local, remote)
		}
	}
}

func (s *Scheduler) addRemoteAddress(remote *net.UDPAddr) {
	s.connection.log(logTypeMultipath, "Adding remote address %v", *remote)
	s.remoteAddrs[remote.String()] = remote
	s.addressHelper.lockAddresses.RLock()
	s.connection.log(logTypeMutex, "locked: ", util.Tracer())
	defer s.addressHelper.lockAddresses.RUnlock()
	defer s.connection.log(logTypeMutex, "unlocked: ", util.Tracer())
	for _, local := range s.localAddrs {
		if isSameVersion(local, remote) {
			s.newPath(local, remote)
		}
	}
	remoteAdded := true
	if remoteAdded {
		//For breakpoints only
	}
}

func (s *Scheduler) removeAddress(address string) {
	if s.containsBlocking(address, remote) {
		s.delete(address, remote)
		s.connection.log(logTypeMultipath, "Deleted remote address %v", address)
	}
	if s.containsBlocking(address, local) {
		s.delete(address, local)
		s.connection.log(logTypeMultipath, "Deleted local address %v", address)
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
	s.connection.log(logTypeMutex, "locked: ", util.Tracer())
	defer s.addressHelper.lockAddresses.RUnlock()
	defer s.lockRemote.RUnlock()
	defer s.connection.log(logTypeMutex, "unlocked: ", util.Tracer())
	for _, local := range s.localAddrs {
		for _, remote := range s.remoteAddrs {
			if isSameVersion(local, remote) {
				s.newPath(local, remote)
			}
		}
	}
	s.connection.log(logTypeMultipath, "First flight paths initialized")
	s.isInitialized = true
}

func (s *Scheduler) removePath(pathId uint32) {
	delete(s.paths, pathId)
	s.connection.log(logTypeMultipath, "Removed path %v", pathId)
}

func (s *Scheduler) ListenOnChannel() {
	s.addressHelper.Subscribe(s.addrChan)
	s.connection.log(logTypeMultipath, "Subscribed to Address Helper")
	go func() {
		oldTime := time.Now().Second()
		for {
			if s.connection.state == StateEstablished {
				addr := <-s.addrChan
				if !s.containsBlocking(addr, local) {
					s.write(addr)
					s.connection.sendFrame(s.assembleAddrModFrame(kAddAddress, addr))
				} else {
					s.delete(addr, local)
					s.connection.sendFrame(s.assembleAddrModFrame(kDeleteAddress, addr))
				}
			} else {
				if time.Now().Second()-oldTime == 10 {

					s.connection.log(logTypeMultipath, "Waiting for connection establishment", util.Tracer())
					oldTime = time.Now().Second()
				}
			}
		}
	}()
}

func (s *Scheduler) assebleAddrArrayFrame() []*frame {
	arr := make([]net.UDPAddr, 0)
	s.addressHelper.lockAddresses.RLock()
	s.connection.log(logTypeMutex, "locked: ", util.Tracer())
	defer s.addressHelper.lockAddresses.RUnlock()
	defer s.connection.log(logTypeMutex, "unlocked: ", util.Tracer())
	for _, v := range s.localAddrs {
		arr = append(arr, *v)
	}
	frames := make([]*frame, 0)
	frame := newAddrArrayFrame(arr)
	frames = append(frames, frame)
	s.connection.log(logTypeMultipath, "Assembled frame", frame)
	return frames
}

func (s *Scheduler) assembleAddrModFrame(delete operation, addr string) *frame {
	frame := newAddrModFrame(delete, addr)
	s.connection.log(logTypeMultipath, "Assembled frame", frame)
	return frame
}

func (s *Scheduler) assembleOwdFrame(pathId uint32) *frame {
	frame := newOwdFrame(pathId)
	s.connection.log(logTypeMultipath, "Assembled frame", frame)
	return frame
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

func isSameVersion(local, remote *net.UDPAddr) bool {
	if local.IP.To4() == nil && remote.IP.To4() == nil {
		return true
	}

	if local.IP.To4() != nil && remote.IP.To4() != nil {
		return true
	}
	return false
}

func (s *Scheduler) containsBlocking(addr string, direcion direcionAddr) bool {
	var contains bool
	if direcion == local {
		s.addressHelper.lockAddresses.RLock()
		s.connection.log(logTypeMutex, "locked: ", util.Tracer())
		defer s.addressHelper.lockAddresses.RUnlock()
		defer s.connection.log(logTypeMutex, "unlocked: ", util.Tracer())
		_, contains = s.localAddrs[addr]
	} else if direcion == remote {
		s.lockRemote.Lock()
		s.connection.log(logTypeMutex, "locked: ", util.Tracer())
		defer s.lockRemote.Unlock()
		defer s.connection.log(logTypeMutex, "unlocked: ", util.Tracer())
		_, contains = s.remoteAddrs[addr]
	}
	return contains
}

func (s *Scheduler) delete(addr string, direction direcionAddr) {
	for key, path := range s.paths {
		if path.contains(addr) {
			s.deletePath(key)
		}
	}
	if direction == local {
		s.addressHelper.lockAddresses.Lock()
		s.connection.log(logTypeMutex, "locked: ", util.Tracer())
		defer s.addressHelper.lockAddresses.Unlock()
		defer s.connection.log(logTypeMutex, "unlocked: ", util.Tracer())
		delete(s.localAddrs, addr)
	}
	if direction == remote {
		s.lockRemote.Lock()
		s.connection.log(logTypeMutex, "locked: ", util.Tracer())
		defer s.lockRemote.Unlock()
		defer s.connection.log(logTypeMutex, "unlocked: ", util.Tracer())
		delete(s.remoteAddrs, addr)
	}
}

func (s *Scheduler) deletePath(pathId uint32) {
	s.lockPaths.Lock()
	s.connection.log(logTypeMutex, "locked: ", util.Tracer())
	defer s.lockPaths.Unlock()
	defer s.connection.log(logTypeMutex, "unlocked: ", util.Tracer())
	delete(s.paths, pathId)
}

func (s *Scheduler) write(addr string) {
	s.addressHelper.lockAddresses.Lock()
	s.connection.log(logTypeMutex, "locked: ", util.Tracer())
	defer s.addressHelper.lockAddresses.Unlock()
	defer s.connection.log(logTypeMutex, "unlocked: ", util.Tracer())
	s.localAddrsBool[addr] = false
}

func (s *Scheduler) setOwd(pathID uint32, owd int64){
	s.lockPaths.Lock()
	s.connection.log(logTypeMutex, "locked: ", util.Tracer())
	defer s.lockPaths.Unlock()
	defer s.connection.log(logTypeMutex, "unlocked: ", util.Tracer())
	s.paths[pathID].setOwd(owd)
}

func (s *Scheduler) measurePathsRunner() {
	go func() {
		for {
			if s.connection.state == StateEstablished {
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

func (s *Scheduler) measurePath(path *Path) {
	s.lockPaths.RLock()
	s.connection.log(logTypeMutex, "locked: ", util.Tracer())
	defer s.lockPaths.RUnlock()
	defer s.connection.log(logTypeMutex, "unlocked: ", util.Tracer())
	s.connection.sendFrame(s.assembleOwdFrame(path.pathID))
	s.connection.log(logTypeMultipath, "Sent OWD Frame for %v", path.pathID)
}

func (s *Scheduler) sumUpWeights() {
	s.lockPaths.RLock()
	s.connection.log(logTypeMutex, "locked: ", util.Tracer())
	defer s.lockPaths.RUnlock()
	defer s.connection.log(logTypeMutex, "unlocked: ", util.Tracer())
	s.totalPathWeight = 0
	for _,path := range s.paths{
		s.totalPathWeight += path.weight
	}
}

func (s *Scheduler) weightedSelect() (*Path, error){
	s.lockPaths.RLock()
	s.connection.log(logTypeMutex, "locked: ", util.Tracer())
	defer s.lockPaths.RUnlock()
	defer s.connection.log(logTypeMutex, "unlocked: ", util.Tracer())
	rand.Seed(time.Now().UnixNano())
	r := rand.Intn(s.totalPathWeight)
	for _, path := range s.paths {
		r -= path.weight
		if r <= 0 {
			return path, nil
		}
	}
	return &Path{}, errors.New("No path selected")
}

func (s *Scheduler) weighPathsRunner() {
	go func() {
		for {
			if s.connection.state == StateEstablished {
				s.weighPaths()
			}
			time.Sleep(100 * time.Millisecond)		}
	}()
}

func (s *Scheduler) weighPaths() {s.lockPaths.Lock()
	for _,path := range s.paths{
		s.weighPath(path)
	}
	s.sumUpWeights()
}

func (s *Scheduler) weighPath(path *Path){
	s.lockPaths.Lock()
	s.connection.log(logTypeMutex, "locked: ", util.Tracer())
	defer s.lockPaths.Unlock()
	defer s.connection.log(logTypeMutex, "unlocked: ", util.Tracer())
	if path.owd < s.calculateAverageOwd(){
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
func (s *Scheduler) calculateAverageOwd() uint64{
	var aOwd uint64
	for _,path := range s.paths{
		aOwd += path.owd
	}
	aOwd = aOwd / uint64(len(s.paths))
	s.connection.log(logTypeMultipath, "Average OWD is: %v", aOwd)
	return aOwd
}