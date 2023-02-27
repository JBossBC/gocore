package netpoll

type Poll interface {
	Wait() error
	Close() error
	Trigger() error
	Control(operator *FDOperator, event PollEvent) error
	Alloc() (operator *FDOperator)
	Free(operator *FDOperator)
}
type PollEvent int

const (
	PollReadable    PollEvent = 0x1
	PollWritable    PollEvent = 0x2
	PollDetach      PollEvent = 0x3
	PollModReadable PollEvent = 0x4
	PollR2RW        PollEvent = 0x5
	PollRW2R        PollEvent = 0x6
)
