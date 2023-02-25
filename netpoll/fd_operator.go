package netpoll

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
