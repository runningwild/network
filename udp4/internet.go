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

func (n *Network) Dial(laddr, raddr network.Addr) (network.Conn, error) {
	laddrResolved, err := net.ResolveUDPAddr(laddr.Network(), laddr.String())
	if err != nil {
		return nil, err
	}
	raddrResolved, err := net.ResolveUDPAddr(raddr.Network(), raddr.String())
	if err != nil {
		return nil, err
	}
	conn, err := net.DialUDP("udp4", laddrResolved, raddrResolved)
	return wrapConn{conn}, err
}
func (n *Network) Listen(laddr network.Addr) (network.Conn, error) {
	laddrResolved, err := net.ResolveUDPAddr(laddr.Network(), laddr.String())
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
	return wc.RemoteAddr()
}
