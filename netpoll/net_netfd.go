package netpoll

import "C"
import (
	"context"
	"errors"
	"net"
	"os"
	"runtime"
	"syscall"
)

type netFD struct {
	fd            int
	pd            *pollDesc
	closed        uint32
	isStream      bool
	zeroReadIsEOF bool
	family        int
	sotype        int
	isConnected   bool
	network       string
	localAddr     net.Addr
	remoteAddr    net.Addr
}

func newNetFD(fd, family, sotype int, net string) *netFD {
	var ret = &netFD{}
	ret.fd = fd
	ret.network = net
	ret.sotype = sotype
	ret.isStream = sotype == syscall.SOCK_STREAM
	//TODO how to use
	ret.zeroReadIsEOF = sotype != syscall.SOCK_DGRAM && sotype != syscall.SOCK_RAW
	return ret
}

func (c *netFD) dial(ctx context.Context, laddr, raddr socketaddr) (err error) {
	var lsa syscall.Sockaddr
	if laddr != nil {
		if lsa, err = laddr.sockaddr(c.family); err != nil {
			return err

		} else if lsa != nil {
			if err = syscall.Bind(c.fd, lsa); err != nil {
				return os.NewSyscallError("bind", err)
			}
		}
	}
	var rsa syscall.Sockaddr
	var crsa syscall.Sockaddr
	if raddr != nil {
		if rsa, err = raddr.sockaddr(c.family); err != nil {
			return err
		}
	}
	if crsa, err = c.connect(ctx, lsa, rsa); err != nil {
		return err
	}
	c.isConnected = true
	lsa, _ = syscall.Getsockname(c.fd)
	c.localAddr = sockaddrToAddr(lsa)
	if crsa != nil {
		c.remoteAddr = sockaddrToAddr(crsa)
		//TODO how to use
	} else if crsa, _ = syscall.Getpeername(c.fd); crsa != nil {
		c.remoteAddr = sockaddrToAddr(crsa)
	} else {
		c.remoteAddr = sockaddrToAddr(rsa)
	}
	return nil
}
func (c *netFD) connect(ctx context.Context, la, ra syscall.Sockaddr) (rsa syscall.Sockaddr, retErr error) {
	switch err := syscall.Connect(c.FD, ra); err {
	case syscall.EINPROGRESS, syscall.EALREADY, syscall.EINTR:
	case nil, syscall.EISCONN:
		select {
		case <-ctx.Done():
			return nil, mapErr(ctx.Err())
		default:

		}
		return nil, nil
	case syscall.EINVAL:
		if runtime.GOOS == "solaris" {
			return nil, nil
		}
		fallthrough
	default:
		return nil, os.NewSyscallError("connect", err)
	}
	c.pd = netPollDesc(c.fd)
	for {
		if err := c.pd.WaitWrite(ctx); err != nil {
			return nil, err
		}
		//TODO SO_ERROR is cant exist in 1.18
		nerr, err := syscall.GetsockoptInt(c.fd, syscall.SOL_SOCKET)
		if err != nil {
			return nil, os.NewSyscallError("getsockopt", err)
		}
		switch err := syscall.Errno(nerr); err {
		case syscall.EINPROGRESS, syscall.EALREADY, syscall.EINTR:
		case syscall.EISCONN:
			return nil, nil
		case syscall.Errno(0):
			if rsa, err := syscall.Getpeername(c.fd); err != nil {
				return rsa, err
			}
		default:
			return nil, os.NewSyscallError("connect", err)

		}
	}
}

var (
	errMissingAddress = errors.New("missing address")
	errCanceled       = errors.New("operation was canceled")
	errIOTimeout      = errors.New("i/o timeout")
)

func mapErr(err error) error {
	switch err {
	case context.Canceled:
		return errCanceled
	case context.DeadlineExceeded:
		return errIOTimeout
	default:
		return err
	}
}