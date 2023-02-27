package netpoll

func (c *Connection) onHup(p Poll) error {
	if c.closeBy(poller) {
		c.triggerRead()
		c.triggerWrite(ErrConnClosed)
		var onConnect, _ = c.on
	}
}
