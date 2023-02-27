package netpoll

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"
)

func newServer(ln Listener, ops *options, onQuit func(err error)) *server {
	return &server{
		ln:     ln,
		opts:   ops,
		onQuit: onQuit,
	}
}

type server struct {
	operator    FDOperator
	ln          Listener
	opts        *options
	onQuit      func(err error)
	connections sync.Map
}

func (s *server) Run() (err error) {
	s.operator = FDOperator{FD: s.ln.Fd(), OnRead: s.OnRead, OnHup: s.OnHup}
	s.operator.poll = pollmanager.Pick()
	err = s.operator.Control(PollReadable)
	if err != nil {
		s.onQuit(err)
	}
	return err
}
func (s *server) Close(ctx context.Context) error {
	s.operator.Control(PollDetach)
	s.ln.Close()
	var ticker = time.NewTicker(time.Second)
	defer ticker.Stop()
	var hasConn bool
	for {
		hasConn = false
		s.connections.Range(func(key, value any) bool {
			var conn, ok = value.(gracefulExit)
			if !ok || conn.isIdle() {
				value.(Connection).Close()
			}
			hasConn = true
			return true
		})
		if !hasConn {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			continue
		}
	}
}
func (s *server) OnRead(p Poll) error {
	conn, err := s.ln.Accept()
	if err != nil {
		if strings.Contains(err.Error(), "closed") {
			s.operator.Control(PollDetach)
			s.onQuit(err)
			return err
		}
		logger.Println("NETPOLL: accept conn failed:", err.Error())
		return err
	}
	if conn == nil {
		return nil
	}
	var connection = &connection{}
	connection.init(conn.(Conn), s.opts)
	if !connection.IsActive() {
		return nil
	}
	var fd = conn.(Conn).FD()
	connection.AddCloseCallback(func(connection Connection) error {
		s.connections.Delete(fd)
		return nil
	})
	s.connections.Store(fd, connection)
	connection.onConnect()
	return nil
}
func (s *server) OnHup(p Poll) error {
	s.onQuit(errors.New("listener close"))
	return nil
}
