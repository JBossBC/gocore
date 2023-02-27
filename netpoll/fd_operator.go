package netpoll

import (
	"runtime"
	"sync/atomic"
)

type FDOperator struct {
	FD        int
	OnRead    func(p Poll) error
	OnWrite   func(p Poll) error
	OnHup     func(p Poll) error
	Inputs    func(vs [][]byte) (rs [][]byte)
	InputAck  func(n int) (err error)
	Outputs   func(vs [][]byte) (rs [][]byte, supportZeroCopy bool)
	OutputAck func(n int) (err error)
	poll      Poll
	next      *FDOperator
	state     int32
	index     int32
}

func (op *FDOperator) Control(event PollEvent) error {
	return op.poll.Control(op, event)
}
func (op *FDOperator) Free() {
	op.poll.Free(op)
}
func (op *FDOperator) do() (can bool) {
	return atomic.CompareAndSwapInt32(&op.state, 1, 2)
}
func (op *FDOperator) done() {
	atomic.StoreInt32(&op.state, 1)
}
func (op *FDOperator) inuse() {
	for !atomic.CompareAndSwapInt32(&op.state, 0, 1) {
		if atomic.LoadInt32(&op.state) == 1 {
			return
		}
		runtime.Gosched()
	}
}
func (op *FDOperator) unused() {
	for !atomic.CompareAndSwapInt32(&op.state, 1, 0) {
		if atomic.LoadInt32(&op.state) == 0 {
			return
		}
		runtime.Gosched()
	}

}
func (op *FDOperator) isUnused() bool {
	return atomic.LoadInt32(&op.state) == 0
}
func (op *FDOperator) reset() {
	op.FD = 0
	op.OnRead, op.OnWrite, op.OnHup = nil, nil, nil
	op.Inputs, op.InputAck = nil, nil
	op.Outputs, op.OutputAck = nil, nil
	op.poll = nil
}
