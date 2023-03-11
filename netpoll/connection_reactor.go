package netpoll

import "sync/atomic"

func (c *Connection) onHup(p Poll) error {
	if c.closeBy(poller) {
		c.triggerRead()
		c.triggerWrite(ErrConnClosed)
		var onConnect, _ = c.on
	}
}
func (c *connection) onClose() error {
	if c.closeBy(user) {
		c.triggerRead()
		c.triggerWrite(ErrConnClosed)
		c.closeCallback(true)
		return nil
	}
	if c.isCloseBy(poller) {
		c.closeCallback(true)
	}
	return nil
}

func (c *connection) closeBuffer() {
	var onConnect, _ = c.onConnectCallback.Load().(OnConnect)
	var onRequest, _ = c.onRequestCallback.Load().(OnRequest)
	if c.inputBuffer.Len() == 0 || onConnect != nil || onRequest != nil {
		c.inputBuffer.Close()
		barrierPool.Put(c.inputBarrier)
	}
	if c.outputBuffer.Len() == 0 || onConnect != nil || onRequest != nil {
		c.outputBuffer.Close()
		barrierPool.Put(c.outputBarrier)
	}
}
func (c *connection) inputs(vs [][]byte) (rs [][]byte) {
	vs[0] = c.inputBuffer.book(c.bookSize, c.maxSize)
	return vs[:1]
}
func (c *connection) inputAck(n int) (err error) {
	if n <= 0 {
		c.inputBuffer.bookAck(0)
		return nil
	}

	// Auto size bookSize.
	if n == c.bookSize && c.bookSize < mallocMax {
		c.bookSize <<= 1
	}

	length, _ := c.inputBuffer.bookAck(n)
	if c.maxSize < length {
		c.maxSize = length
	}
	if c.maxSize > mallocMax {
		c.maxSize = mallocMax
	}

	var needTrigger = true
	if length == n { // first start onRequest
		needTrigger = c.onRequest()
	}
	if needTrigger && length >= int(atomic.LoadInt64(&c.waitReadSize)) {
		c.triggerRead()
	}
	return nil
}
func (c *connection) outputs(vs [][]byte) (rs [][]byte, supportZeroCopy bool) {
	if c.outputBuffer.IsEmpty() {
		c.rw2r()
		return rs, c.supportZeroCopy
	}
	rs = c.outputBuffer.GetBytes(vs)
	return rs, c.supportZeroCopy
}
func (c *connection) outputAck(n int) (err error) {
	if n > 0 {
		c.outputBuffer.Skip(n)
		c.outputBuffer.Release()
	}
	if c.outputBuffer.IsEmpty() {
		c.rw2r()
	}
	return nil
}

func (c *connection) rw2r() {
	c.operator.Control(PollRW2R)
	c.triggerWrite(nil)
}
