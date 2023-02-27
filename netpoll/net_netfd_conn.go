package netpoll

import (
	"net"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

var _ Conn = &netFD{}

func (c *netFD) FD() (fd int) {
	return c.fd
}

func (c *netFD) Read(b []byte) (n int, err error) {
	n, err = syscall.Read(c.fd, b)
	if err != nil {
		if err == syscall.EAGAIN || err == syscall.EINTR {
			return 0, nil
		}
	}
	return n, err
}
func (c *netFD) Write(b []byte) (n int, err error) {
	n, err = syscall.Write(c.fd, b)
	if err != nil {
		if err == syscall.EAGAIN {
			return 0, nil
		}
	}
	return n, err
}
func (c *netFD) Close() (err error) {
	if atomic.AddInt32(&c.closed, 1) != 1 {
		return nil
	}
	if c.fd > 0 {
		err = syscall.Close(c.fd)
		if err != nil {
			logger.Printf("NETPOLL: netFD[%d] close error: %s", c.fd, err.Error())
		}
	}
	return err
}

func (c *netFD) LocalAddr() (addr net.Addr) {
	return c.localAddr
}
func (c *netFD) RemoteAddr() (addr net.Addr) {
	return c.remoteAddr
}

func (c *netFD) SetKeepAlive(second int) error {
	if !strings.HasPrefix(c.network, "tcp") {
		return nil
	}
	if second > 0 {
		return SetKeepAlive(c.fd, second)
	}
	return nil
}
func (c *netFD) SetDeadline(t time.Time) error {
	return Exception(ErrUnsupported, "SetDeadline")
}
func (c *netFD) SetReadDeadline(t time.Time) error {
	return Exception(ErrUnsupported, "SetReadDeadline")
}
func (c *netFD) SetWriteDeadline(t time.Time) error {
	return Exception(ErrUnsupported, "SetWriteDeadline")
}
