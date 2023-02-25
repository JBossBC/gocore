package netpoll

import (
	"net"
	"time"
)

type CloseCallBack func(connection Connection) error
type Connection interface {
	net.Conn
	Reader() Reader
	Writer() Writer
	IsActive() bool
	SetReadTimeout(timeout time.Duration) error
	SetWriteTimeout(timeout time.Duration) error
	SetOnRequest(on OnRequest) error
	AddCloseCallback(callback CloseCallBack) error
}
type Listener interface {
	net.Listener
	Fd() (fd int)
}
type Conn interface {
	net.Conn
	FD() (fd int)
}
type Dialer interface {
	DialConnection(network, address string, timeout time.Duration) (connection Connection, err error)
	DialTimeout(network, adress string, timeout time.Duration) (conn net.Conn, err error)
}
