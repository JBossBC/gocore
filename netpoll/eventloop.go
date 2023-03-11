package netpoll

import (
	"context"
	"net"
)

type EventLoop interface {
	Server(ln net.Listener) error
	Shutdown(ctx context.Context) error
}

type OnPrepare func(connection Connection) context.Context
type OnConnect func(ctx context.Context, connection Connection) context.Context
type OnRequest func(ctx context.Context, connection Connection) error
