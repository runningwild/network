package udp

import (
	"fmt"
	"github.com/runningwild/network"
	"net"
)

func init() {
	network.RegisterInternet("udp4", func() network.Internet { return MakeInternet() })
}

func MakeInternet() network.Internet {
	return &Internet{}
}

type Internet struct{}

func (internet *Internet) MakeNetwork() (string, network.Network) {
	return "", &Network{}
}

type Network struct{}

func simpleResolve(addr network.Addr) (*net.UDPAddr, error) {
	var stringNetwork, stringAddr string
	if addr == nil {
		stringNetwork = "udp4"
		stringAddr = ""
	} else {
		stringNetwork = addr.Network()
		stringAddr = addr.String()
	}
	addrResolved, err := net.ResolveUDPAddr(stringNetwork, stringAddr)
	if err != nil {
		return nil, err
	}
	return addrResolved, nil
}

func (n *Network) Dial(laddr, raddr network.Addr) (network.Conn, error) {
	laddrResolved, err := simpleResolve(laddr)
	if err != nil {
		return nil, err
	}
	raddrResolved, err := simpleResolve(raddr)
	if err != nil {
		return nil, err
	}
	conn, err := net.DialUDP("udp4", laddrResolved, raddrResolved)
	return wrapConn{conn}, err
}
func (n *Network) Listen(laddr network.Addr) (network.Conn, error) {
	laddrResolved, err := simpleResolve(laddr)
	if err != nil {
		return nil, err
	}
	conn, err := net.ListenUDP("udp4", laddrResolved)
	return wrapConn{conn}, err
}
func (n *Network) Forward(external int, internal network.Addr) error {
	return fmt.Errorf("UDP4 network cannot do forwarding.")
}
func (n *Network) Resolve(host string, port int) (network.Addr, error) {
	return net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%d", host, port))
}

type wrapConn struct {
	*net.UDPConn
}

func (wc wrapConn) LocalAddr() network.Addr {
	return wc.UDPConn.LocalAddr()
}
func (wc wrapConn) ReadFrom(b []byte) (int, network.Addr, error) {
	return wc.UDPConn.ReadFrom(b)
}
func (wc wrapConn) RemoteAddr() network.Addr {
	return wc.UDPConn.RemoteAddr()
}
func (wc wrapConn) WriteTo(b []byte, raddr network.Addr) (int, error) {
	raddrResolved, err := simpleResolve(raddr)
	if err != nil {
		return 0, err
	}
	return wc.UDPConn.WriteTo(b, raddrResolved)
}
