package fake

import (
	"fmt"
	"github.com/runningwild/network"
	"math/rand"
	"os"
	"sync"
	"time"
)

type Internet struct {
	packetLoss    float64
	maxPacketSize int

	nets struct {
		sync.Mutex
		nextHost      int
		hostToNetwork map[int]*Network
	}
}

func (in *Internet) SetPacketLoss(packetLoss float64) {
	in.packetLoss = packetLoss
}

func (in *Internet) SetMaxPacketSize(maxPacketSize int) {
	in.maxPacketSize = maxPacketSize
}

func (in *Internet) SendPacket(p packet) {
	if rand.Float64() < in.packetLoss {
		return
	}
	if in.maxPacketSize > 0 && len(p.Data) > in.maxPacketSize {
		p.Data = p.Data[0:in.maxPacketSize]
	}
	in.nets.Lock()
	defer in.nets.Unlock()
	network, ok := in.nets.hostToNetwork[p.Dst.host]
	if !ok {
		return
	}
	go func() {
		network.fromInternet <- p
	}()
}

func MakeInternet() *Internet {
	var in Internet
	in.nets.hostToNetwork = make(map[int]*Network)
	return &in
}

func init() {
	network.RegisterInternet("fake", func() network.Internet { return MakeInternet() })
}

func (in *Internet) MakeNetwork() (string, network.Network) {
	in.nets.Lock()
	defer in.nets.Unlock()
	in.nets.nextHost++
	network := makeNetwork(in, in.nets.nextHost)
	in.nets.hostToNetwork[in.nets.nextHost] = network
	return fmt.Sprintf("%d", in.nets.nextHost), network
}

type Addr struct {
	host, port int
}

func (a Addr) Network() string {
	return "fake"
}
func (a Addr) String() string {
	return fmt.Sprintf("%d:%d", a.host, a.port)
}
func ResolveAddr(addr string) (Addr, error) {
	var a Addr
	fmt.Sscanf(addr, "%d:%d", &a.host, &a.port)
	return a, nil
}

type packet struct {
	Src, Dst Addr
	Data     []byte
}

type Network struct {
	internet *Internet

	// Assigned by the internet
	host int

	// Provides an arbitrary mapping from internal port to external port to
	// simulate the network address translation that happens on routers and
	// whatnot.
	nat struct {
		sync.RWMutex

		// next port to use when assigning unused ports
		next int

		// Maps internal addr to external port.
		outward map[Addr]int

		// Maps external port to internal addr.
		inward map[int]Addr
	}

	conns struct {
		sync.RWMutex

		addrToConn map[Addr]*Conn
	}

	toInternet   chan packet
	fromInternet chan packet
}

func makeNetwork(in *Internet, host int) *Network {
	var net Network
	net.internet = in
	net.host = host
	net.nat.outward = make(map[Addr]int)
	net.nat.inward = make(map[int]Addr)
	net.conns.addrToConn = make(map[Addr]*Conn)
	net.toInternet = make(chan packet)
	net.fromInternet = make(chan packet)
	go net.routine()
	return &net
}

// nextPort() returns an unused port external facing port for this Network.
// This method assumes that a writer lock on net.nat is held.
func (net *Network) nextPort() int {
	for {
		if _, ok := net.nat.inward[net.nat.next]; ok {
			net.nat.next++
			continue
		}
		break
	}
	return net.nat.next
}

func (net *Network) routine() {
	for {
		select {
		case p := <-net.toInternet:
			// When a connection wants to send a packet across the internet, first
			// make sure that connection is mentioned in the nat, then replace the
			// src address on the packet to match the external addr.
			net.nat.Lock()
			if _, ok := net.nat.outward[p.Src]; !ok {
				next := net.nextPort()
				externalPort := next
				net.nat.outward[p.Src] = externalPort
				net.nat.inward[externalPort] = p.Src
			}
			p.Src.port = net.nat.outward[p.Src]
			net.nat.Unlock()
			net.internet.SendPacket(p)

		case p := <-net.fromInternet:
			// When receiving a packet check for the dst addr in the nat, if
			// it is not there then drop the packet, otherwise change the dst
			// addr and send it to the connection.
			net.nat.RLock()
			internal, ok := net.nat.inward[p.Dst.port]
			net.nat.RUnlock()
			if !ok {
				continue
			}
			p.Dst = internal
			net.conns.RLock()
			conn, ok := net.conns.addrToConn[p.Dst]
			net.conns.RUnlock()
			if !ok {
				continue
			}
			go func(c *Conn, p packet) {
				c.fromInternet <- p
			}(conn, p)
		}
	}
}

func (net *Network) Resolve(host string, port int) (network.Addr, error) {
	var hostInt int
	if host == "" {
		hostInt = net.host
	} else {
		_, err := fmt.Sscanf(host, "%d", &hostInt)
		if err != nil {
			return nil, err
		}
	}
	if port == 0 {
		net.conns.Lock()
		for {
			port = rand.Intn(1000)
			_, ok := net.conns.addrToConn[Addr{hostInt, port}]
			if !ok {
				break
			}
		}
		net.conns.Unlock()
	}
	return Addr{hostInt, port}, nil
}

func (net *Network) Forward(external int, internal network.Addr) error {
	internalConv, ok := internal.(Addr)
	if !ok {
		return fmt.Errorf("Expected a fake addr, got %v.", internal)
	}
	net.nat.Lock()
	defer net.nat.Unlock()
	if _, ok := net.nat.inward[external]; ok {
		return fmt.Errorf("Port %d is already taken.", external)
	}
	net.nat.inward[external] = internalConv
	net.nat.outward[internalConv] = external
	return nil
}
func (net *Network) Dial(laddr, raddr network.Addr) (network.Conn, error) {
	var laddrConv Addr
	if laddr == nil {
		laddrConv.host = net.host
		net.conns.Lock()
		for {
			laddrConv.port = rand.Intn(1000)
			_, ok := net.conns.addrToConn[laddrConv]
			if !ok {
				break
			}
		}
		net.conns.Unlock()
	} else {
		var ok bool
		laddrConv, ok = laddr.(Addr)
		if !ok {
			return nil, fmt.Errorf("Expected a fake addr, got %v.", laddr)
		}
	}
	raddrConv, ok := raddr.(Addr)
	if !ok {
		return nil, fmt.Errorf("Expected a fake addr, got %v.", raddr)
	}
	net.nat.Lock()
	defer net.nat.Unlock()
	if laddrConv.port == 0 {
		laddrConv.port = net.nextPort()
	}
	if _, ok := net.nat.inward[laddrConv.port]; ok {
		return nil, fmt.Errorf("Failed to dial, port %d already in used.", laddrConv.port)
	}
	next := net.nextPort()
	net.nat.inward[next] = laddrConv
	net.nat.outward[laddrConv] = next
	conn := Conn{
		net:          net,
		localAddr:    laddrConv,
		remoteAddr:   raddrConv,
		fromInternet: make(chan packet),
	}
	net.conns.Lock()
	net.conns.addrToConn[laddrConv] = &conn
	net.conns.Unlock()
	return &conn, nil
}
func (net *Network) Listen(laddr network.Addr) (network.Conn, error) {
	laddrConv, ok := laddr.(Addr)
	if !ok {
		return nil, fmt.Errorf("Expected a fake addr, got %v.", laddr)
	}
	net.nat.Lock()
	defer net.nat.Unlock()
	if _, ok := net.nat.inward[laddrConv.port]; ok {
		return nil, fmt.Errorf("Failed to listen, port %d already in used.", laddrConv.port)
	}
	next := net.nextPort()
	net.nat.inward[next] = laddrConv
	conn := Conn{
		net:          net,
		localAddr:    laddrConv,
		remoteAddr:   Addr{},
		fromInternet: make(chan packet),
	}
	net.conns.Lock()
	net.conns.addrToConn[laddrConv] = &conn
	net.conns.Unlock()
	return &conn, nil
}

type Conn struct {
	net        *Network
	localAddr  Addr
	remoteAddr Addr

	fromInternet chan packet
}

func (c *Conn) Close() error {
	return nil
}
func (c *Conn) File() (f *os.File, err error) {
	return nil, nil
}
func (c *Conn) LocalAddr() network.Addr {
	return c.localAddr
}
func (c *Conn) ReadFrom(b []byte) (int, network.Addr, error) {
	p := <-c.fromInternet
	copy(b, p.Data)
	n := len(p.Data)
	if len(b) < n {
		n = len(b)
	}
	return n, p.Src, nil
}
func (c *Conn) Read(b []byte) (int, error) {
	n, _, err := c.ReadFrom(b)
	return n, err
}
func (c *Conn) RemoteAddr() network.Addr {
	return c.remoteAddr
}
func (c *Conn) SetDeadline(t time.Time) error {
	return nil
}
func (c *Conn) SetReadBuffer(bytes int) error {
	return nil
}
func (c *Conn) SetReadDeadline(t time.Time) error {
	return nil
}
func (c *Conn) SetWriteBuffer(bytes int) error {
	return nil
}
func (c *Conn) SetWriteDeadline(t time.Time) error {
	return nil
}
func (c *Conn) Write(b []byte) (int, error) {
	return c.WriteTo(b, c.remoteAddr)
}
func (c *Conn) WriteTo(b []byte, raddr network.Addr) (int, error) {
	buffer := make([]byte, len(b))
	copy(buffer, b)
	raddrResolved, err := ResolveAddr(raddr.String())
	if err != nil {
		return 0, err
	}
	go func() {
		c.net.toInternet <- packet{
			Src:  c.localAddr,
			Dst:  raddrResolved,
			Data: buffer,
		}
	}()
	return len(buffer), nil
}
