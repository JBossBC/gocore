//go:build darwin || netbsd || freebsd || openbsd || dragonfly || linux
// +build darwin netbsd freebsd openbsd dragonfly linux

package netpoll

import (
	"errors"
	"net"
	"os"
	"syscall"
)

var _ net.Listener = &listener{}

type listener struct {
	fd    int
	addr  net.Addr
	ln    net.Listener
	pconn net.PacketConn
	file  *os.File
}

func (ln *listener) Accept() (net.Conn, error) {
	// udp
	if ln.pconn != nil {
		return ln.UDPAccept()
	}
	var fd, sa, err = syscall.Accept(ln.fd)
	if err != nil {
		//how to use
		if err == syscall.EAGAIN {
			return nil, nil
		}
		return nil, err
	}
	var nfd = &netFD{}
	nfd.fd = fd
	nfd.localAddr = ln.addr
	nfd.network = ln.addr.Network()
	nfd.remoteAddr = sockaddrToAddr(sa)
	return nfd, nil
}
func (ln *listener) UDPAccept() (net.Conn, error) {
	return nil, Exception(ErrUnsupported, "UDP")
}
func (ln *listener) Close() error {
	if ln.fd != 0 {
		syscall.Close(ln.fd)
	}
	if ln.file != nil {
		ln.file.Close()
	}
	if ln.ln != nil {
		ln.ln.Close()
	}
	if ln.pconn != nil {
		ln.pconn.Close()
	}
	return nil
}
func (ln *listener) Addr() net.Addr {
	return ln.addr
}
func (ln *listener) Fd() (fd int) {
	return ln.fd
}
func (ln *listener) parseFD() (err error) {
	switch netln := ln.ln.(type) {
	case *net.TCPListener:
		ln.file, err = netln.File()
	case *net.UnixListener:
		ln.file, err = netln.File()
	default:
		return errors.New("listener type cant support")
	}
	if err != nil {
		return err
	}
	ln.fd = int(ln.file.Fd())
	return nil
}

func CreateListener(network, addr string) (l Listener, err error) {
	if network == "udp" {
		return udpListener(network, addr)
	}
	ln, err := net.Listen(network, addr)
	if err != nil {
		return nil, err
	}
	return ConvertListener(ln)
}
func udpListener(network, addr string) (l Listener, err error) {
	ln := &listener{}
	//TODO how to use
	ln.pconn, err = net.ListenPacket(network, addr)
	if err != nil {
		return nil, err
	}
	ln.addr = ln.pconn.LocalAddr()
	switch pconn := ln.pconn.(type) {
	case *net.UDPConn:
		ln.file, err = pconn.File()
	}
	if err != nil {
		return nil, err
	}
	ln.fd = int(ln.file.Fd())
	//TODO how to use
	return ln, syscall.SetNonblock(ln.fd, true)
}
func ConvertListener(l net.Listener) (nl Listener, err error) {
	if tmp, ok := l.(Listener); ok {
		return tmp, nil
	}
	ln := &listener{}
	ln.ln = l
	ln.addr = l.Addr()
	err = ln.parseFD()
	if err != nil {
		return nil, err
	}
	return ln, syscall.SetNonblock(ln.fd, true)
}
