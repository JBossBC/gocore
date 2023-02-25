package netpoll

import "context"

func newPollDesc(fd int) *pollDesc {

}

type pollDesc struct {
	operator     *FDOperator
	writeTrigger chan struct{}
	closeTrigger chan struct{}
}

func (pd *pollDesc) WaitWrite(ctx context.Context) (err error) {
	defer func() {
		if err != nil {
			pd.operator.Free()
		}
	}()
	if pd.operator.isUnused() {
		if err = pd.operator.Control(PollWritable); err != nil {
			logger.Printf("NETPOLL: pollDESC register operator failed: %v", err)
			return err
		}
	}
	select {
	case <-pd.closeTrigger:
		return Exception(ErrConnClosed, "by peer")
	case <-pd.writeTrigger:
		err = nil
	case <-ctx.Done():
		pd.detach()
		err = mapErr(ctx.Err())
	}
	// double check close trigger
	select{
	case <-pd.closeTrigger:
		return Exception(ErrConnClosed, "by peer")
	default
		return err
	}
}

func(pd *pollDesc)onwrite(p Poll)error{
	select{
	case <-pd.writeTrigger:
		default:
			close(pd.closeTrigger)

	}
	return nil
}
func(pd *pollDesc)onhup(p Poll)error{
	select{
	case<-pd.closeTrigger:
		default:
			close(pd.closeTrigger)
	}
	return nil
}
func(pd *pollDesc)detach(){
	if err:=pd.operator.Control(PollDetach);err!=nil{
		logger.Printf("NETPOLL: pollDesc detach operator failed: %v", err)
	}
}