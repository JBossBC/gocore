//go:build windows
// +build windows

package netpoll

import (
	"net"
	"time"
)

type Option struct {
	f func(*options)
}

type options struct{}

func WithOnPrepare(onPrepare OnPrepare) Option {
	return Option{}
}
func WithOnConnect(onConnect OnConnect) Option {
	return Option{}
}
func WithReadTimeout(timeout time.Duration) Option {
	return Option{}
}
func WithIdleTimeout(timeout time.Duration) Option {
	return Option{}
}
func NewDialer() Dialer {
	return nil
}
func NewEventLoop(onRequest OnRequest, ops ...Option) (EventLoop, error) {
	return nil, nil
}
func ConvertListener(l net.Listener) (nl Listener, err error) {
	return nil, nil
}
func CreateListener(network, addr string) (l Listener, err error) {
	return nil, nil
}
