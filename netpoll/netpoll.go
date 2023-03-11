//go:build darwin || netbsd || freebsd || openbsd || dragonfly || linux
// +build darwin netbsd freebsd openbsd dragonfly linux

package netpoll

import (
	"net"
	"runtime"
	"sync"
)

func NewEventLoop(onRequest OnRequest, ops ...Option) (EventLoop, error) {
	opts := &options{
		OnRequest: onRequest,
	}
	for _, do := range ops {
		do.f(ops)
	}
	return &eventLoop{
		opts: opts,
		stop: make(chan error, 1),
	}, nil
}

type eventLoop struct {
	sync.Mutex
	opts *options
	svr  *server
	stop chan error
}

func (evl *eventLoop) Server(ln net.Listener) error {
	npln, err := ConvertListener(ln)
	if err != nil {
		return err
	}
	evl.Lock()
	evl.svr = newServer(npln, evl.opts, evl.quit)
	evl.svr.Run()
	evl.Unlock()
	err = evl.waitQuit()
	runtime.SetFinalizer(evl, nil)
	return err
}
func (evl *eventLoop) Shutdown(ctx context.Context) error {
	evl.Lock()
	var svr = evl.svr
	evl.svr = nil
	evl.Unlock()

	if svr == nil {
		return nil
	}
	evl.quit(nil)
	return svr.Close(ctx)
}
func (evl *eventLoop) waitQuit() error {
	return <-evl.stop
}

func (evl *eventLoop) quit(err error) {
	select {
	case evl.stop <- err:
	default:
	}
}
