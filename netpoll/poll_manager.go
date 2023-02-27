package netpoll

import (
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
)

func setNumLoops(numLoops int) error {
	return pollmanager.SetNumLoops(numLoops)
}

func setLoadBalance(lb LoadBalance) error {
	return pollmanager.SetLoadBalance(lb)
}
func setLoggerOutput(w io.Writer) {
	logger = log.New(w, "", log.LstdFlags)
}

var pollmanager *manager
var logger *log.Logger

func init() {
	var loops = runtime.GOMAXPROCS(0)/20 + 1
	pollmanager = &manager{}
	pollmanager.SetLoadBalance(RoundRobin)
	pollmanager.SetNumLoops(loops)
	setLoggerOutput(os.Stderr)
}

type manager struct {
	NumLoops int
	balance  loadbalance
	polls    []Poll
}

func (m *manager) SetNumLoops(numLoops int) error {
	if numLoops < 1 {
		return fmt.Errorf("set invalid numLoops[%d]", numLoops)
	}
	if numLoops < m.NumLoops {
		var polls = make([]Poll, numLoops)
		for idx := 0; idx < m.NumLoops; idx++ {
			if idx < numLoops {
				polls[idx] = m.polls[idx]
			} else {
				if err := m.polls[idx].Close(); err != nil {
					logger.Printf("NETPOLL:poller close failed: %v\n", err)
				}
			}
		}
		m.NumLoops = numLoops
		m.polls = polls
		m.balance.Rebalance(m.polls)
		return nil
	}
	m.NumLoops = numLoops
	return m.Run()
}
func (m *manager) SetLoadBalance(lb loadbalance) error {
	if m.balance != nil && m.balance.LoadBalance() == lb {
		return nil
	}
	m.balance = newLoadbalance(lb, m.polls)
	return nil
}
func (m *manager) Close() error {
	for _, poll := range m.polls {
		poll.Close()
	}
	m.NumLoops = 0
	m.balance = nil
	m.polls = nil
	return nil
}
func (m *manager) Run() error {
	for idx := len(m.polls); idx < m.NumLoops; idx++ {
		var poll = openPoll()
		m.polls = append(m.polls, poll)
		go poll.Wait()
	}
	m.balance.Rebalance(m.polls)
	return nil
}

func (m *manager) Reset() error {
	for _, poll := range m.polls {
		poll.Close()
	}
	m.polls = nil
	return m.Run()
}
func (m *manager) Pick() poll {
	return m.balance.Pick()
}
