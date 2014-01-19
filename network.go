package network

import (
	"fmt"
	"os"
	"time"
)

type InternetMaker func() Internet

var internets map[string]InternetMaker

func init() {
	internets = make(map[string]InternetMaker)
}

func RegisterInternet(network string, maker InternetMaker) {
	internets[network] = maker
}

func GetInternet(network string) Internet {
	maker, ok := internets[network]
	if !ok {
		panic(fmt.Sprintf("Asked for internet on network '%s' which wasn't registered.", network))
	}
	return maker()
}

type Internet interface {
	MakeNetwork() (string, Network)
}

type Network interface {
	Dial(laddr, raddr Addr) (Conn, error)
	Listen(laddr Addr) (Conn, error)
	Forward(external int, internal Addr) error
	Resolve(host string, port int) (Addr, error)
}
type Addr interface {
	Network() string // name of the network ('fake' or 'udp')
	String() string  // string form of address
}
type Conn interface {
	Close() error
	File() (f *os.File, err error)
	LocalAddr() Addr
	Read(b []byte) (int, error)
	ReadFrom(b []byte) (int, Addr, error)
	RemoteAddr() Addr
	SetDeadline(t time.Time) error
	SetReadBuffer(bytes int) error
	SetReadDeadline(t time.Time) error
	SetWriteBuffer(bytes int) error
	SetWriteDeadline(t time.Time) error
	Write(b []byte) (int, error)
}
