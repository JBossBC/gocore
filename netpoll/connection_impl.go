package netpoll

import (
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

const (
	defaultZeroCopyTimeoutSec = 60
)

type connection struct {
	netFD
	onEvent
	locker
	operator        *FDOperator
	readTimeout     time.Duration
	readTrigger     chan struct{}
	waitReadSize    int64
	writeTimeout    time.Duration
	writeTimer      *time.Timer
	writeTrigger    chan error
	inputBuffer     *LinkBuffer
	outputBuffer    *LinkBuffer
	inputBarrier    *barrier
	outputBarrier   *barrier
	supportZeroCopy bool
	maxSize         int
	bookSize        int
}

var (
	_ Connection = &connection{}
	_ Reader     = &connection{}
	_ Writer     = &connection{}
)

func (c *connection) Reader() Reader {
	return c
}
func (c *connection) Writer() Writer {
	return c
}
func (c *connection) IsActive() bool {
	return c.isCloseBy(none)
}
func (c *connection) SetIdleTimeout(timeout time.Duration) error {
	if timeout > 0 {
		return c.SetKeepAlive(int(timeout.Seconds()))
	}
	return nil
}
func (c *connection) SetReadTimeout(timeout time.Duration) error {
	if timeout > 0 {
		c.readTimeout = timeout
	}
	return nil
}
func (c *connection) SetWriterTimeout(timeout time.Duration) error {
	if timeout > 0 {
		c.writeTimeout = timeout
	}
	return nil
}

// ------------------------------------------ implement zero-copy reader -----------------------------------------

func (c *connection) Next(n int) (p []byte, err error) {
	if err = c.waitRead(n); err != nil {
		return p, err
	}
	return c.inputBuffer.Next(n)
}

func (c *connection) Peek(n int) (buf []byte, err error) {
	if err = c.waitRead(n); err != nil {
		return buf, err
	}
	return c.inputBuffer.Peek(n)
}
func (c *connection) Skip(n int) (err error) {
	if err = c.waitRead(n); err != nil {
		return err
	}
	return c.inputBuffer.Skip(n)
}
func (c *connection) Release() (err error) {
	if c.inputBuffer.Len() == 0 && c.operator.do() {
		maxSize := c.inputBuffer.calcMaxSize()
		if maxSize > mallocMax {
			maxSize = mallocMax
		}
		if maxSize > c.maxSize {
			c.maxSize = maxSize
		}
		if c.inputBuffer.Len() == 0 {
			c.inputBuffer.resetTail(c.maxSize)
		}
		c.operator.done()
	}
	return c.inputBuffer.Release()
}
func (c *connection) Slice(n int) (r Reader, err error) {
	if err = c.waitRead(n); err != nil {
		return nil, err
	}
	return c.inputBuffer.Slice(n)
}
func (c *connection) Len() (length int) {
	return c.inputBuffer.Len()
}
func (c *connection) Until(delim byte) (line []byte, err error) {
	var n, l int
	for {
		if err = c.waitRead(n + 1); err != nil {
			line, _ = c.inputBuffer.Next(c.inputBuffer.Len())
			return
		}
		l = c.inputBuffer.Len()
		i := c.inputBuffer.indexByte(delim, n)
		if i < 0 {
			n = l
			continue
		}
		return c.Next(i + 1)
	}
}


func(c *connection)ReadString(n int)(s string,err error){
	if err =c.waitRead(n);err!={
		return s,err
	}
	return c.inputBuffer.ReadString(n)
}
func(c *connection)ReadBinary(n int)(p []byte,err error){
	if err=c.waitRead(n);err!=nil{
		return p,err
	}
	return c.inputBuffer.ReadBinary(n)
}
func(c *connection)ReadByte()(b byte,err error){
	if err =c.waitRead(1);err!=nil{
		return b,err
	}
	return c.inputBuffer.ReadByte()
}

// ------------------------------------------ implement zero-copy writer ------------------------------------------

func(c *connection)Malloc(n int)(buf []byte,err error){
	return c.outputBuffer.Malloc(n)
}
func(c *connection)MallocLen()(length int){
	return c.outputBuffer.MallocLen()
}

func(c *connection)Flush()error{
	if !c.IsActive()||!c.lock(flushing){
		return Exception(ErrConnClosed,"when flush")
	}
	defer c.unlock(flushing)
	c.outputBuffer.Flush()
	return c.flush()
}
func(c *connection)MallocAck(n int)(err error){
	return c.outputBuffer.MallocAck(n)
}
func(c *connection)Append(w Writer)(err error){
	return c.outputBuffer.Append(w)
}
func(c *connection)WriteString(s string)(n int,err error){
	return c.outputBuffer.WriteString(s)
}
func(c *connection)WriteBinary(b []byte)(n int,err error){
	return c.outputBuffer.WriteBinary(b)
}
func(c *connection)WriteDirect(p []byte,remainCap int)(err error){
	return c.outputBuffer.WriteDirect(p,remainCap)
}
func(c *connection)WriteByte(b byte)(err error){
	return c.outputBuffer.WriteByte(b)
}
// ------------------------------------------ implement net.Conn ------------------------------------------
// Read behavior is the same as net.Conn, it will return io.EOF if buffer is empty.

func(c *connection)Read(p []byte)(n int,err error){
	l:=len(p)
	if l==0{
		return 0,nil
	}
	if err=c.waitRead(l);err!=nil{
		return 0,err
	}
	if has:=c.inputBuffer.Len();has<l{
		l=has
	}
	src,err:=c.inputBuffer.Next(l)
	n =copy(p,src)
	if err==nil{
		err =c.inputBuffer.Release()
	}
	return n,err
}
func(c *connection)Write(p []byte)(n int,err error){
	if !c.IsActive()||!c.lock(flushing){
		return 0,Exception(ErrConnClosed,"when write")
	}
	defer c.unlock(flushing)
	dst,_=:=c.outputBuffer.Malloc(len(p))
	n=copy(dst,p)
	c.outputBuffer.Flush()
	err =c.flush()
	return n,err
}
func(c *connection)Close()error{
	return c.onClose()
}


// ------------------------------------------ private ------------------------------------------

var barrierPool =sync.Pool{
	New:func()interface{}{
		return &barrier{
			bs:make([][]byte,barriercap),
			ivs:make([]syscall.Iovec,barriercap),
}
	},
}

func( c *connection)init(conn Conn,opts *options)(err error){
	c.readTrigger=make(chan struct{},1)
	c.writeTrigger=make(chan error,1)
	c.bookSize,c.maxSize=block1k/2,pagesize
	c.inputBuffer,c.outputBuffer=NewLinkBuffer(pageSize),NewLinkBuffer()
	c.inputBarrier,c.outputBarrier =barrierPool.Get().(*barrier),barrierPool.Get().(*barrier)
	c.initNetFD(conn)
	c.initFDOperator()
	c.initFinalizer()
	syscall.SetNonblock(c.fd,true)
	switch c.network{
	case "tcp","tcp4","tcp6":
		setTCPNoDelay(c.fd,true)
	}
	if setZeroCopy(c.fd)==nil &&setBlockZeroCopySend(c.fd,defaultZeroCopyTimeoutSec,0)==nil{
		c.supportZeroCopy=true
	}
	return c.onPrepare(opts)
}

func( c *connection)initNetFD(conn Conn){
	if nfd,ok:=conn.(*netFD);ok{
		c.netFD=*nfd
		return
	}
	c.netFD=netFD{
		fd:conn.FD(),
		localAddr: conn.LocalAddr(),
		remoteAddr: conn.RemoteAddr(),
	}
}
func(c *connection)initFDOperator(){
	var op *FDOperator
	if c.pd !=nil &&c.pd.operator!=nil{
		op=c.pd.operator
	}else{
		poll:=pollmanager.Pick()
		op= poll.Alloc()
	}
	op.FD=c.FD()
	op.OnRead,op.OnWrite,op.OnHup=nil,nil,c.onHup
	op.Inputs,op.InputAck=c.inputs,c.inputAck
	op.Outputs,op.OutputAck=c.outputs,c.outputAck
	c.operator=op
}
func(c *connection)initFinalizer(){
	c.AddCloseCallback(func(connection Connection) (err error){
		c.stop(flushing)
		c.stop(finalizing)
		c.operator.Free()
		if err =c.netFD.Close();err!=nil{
			logger.Printf("NETPOLL: netFD close failed: %v",err)
		}
		c.closeBuffer()
		return nil
	})
}
func(c *connection)triggerRead(){
	select{
	case c.readTrigger<- struct{}{}:
	default:

	}
}
func(c *connection)triggerWrite(err error){
	select{
	case c.writeTrigger<-err:
	default:

	}
}
func(c *connection)waitRead(n int)(err error){
	if n<=c.inputBuffer.Len(){
		return nil
	}
	atomic.StoreInt64(&c.waitReadSize,int64(n))
	defer atomic.StoreInt64(&c.waitReadSize,0)
	if c.readTimeout>0{
		return c.waitReadWithTimeout(n)
	}
	for c.inputBuffer.Len()<n{
		if c.IsActive(){
			<-c.readTrigger
			continue
		}
		if atomic.LoadInt32(&c.netFD.closed)==0{
			return c.fill(n)
		}
		return Exception(ErrConnClosed, "wait read")
	}
	return nil
}

func(c *connection)waitReadWithTimeout(n int)(err error){
	if c.readTimer == nil{
		c.readTimer =time.NewTimer(c.readTimeout)
	}else{
		c.readTimer.Reset(c.readTimeout)
	}
	for c.inputBuffer.Len()<n{
		if !c.IsActive(){
			if atomic.LoadInt32(&c.netFD.closed)==0{
				err=c.fill(n)
			}else{
				err=Exception(ErrConnClosed,"wait read")
			}
			break
		}
		select{
		case <-c.readTimer.C:
			if c.inputBuffer.Len()>=n{
				return nil
			}
			return Exception(ErrReadTimeout,c.remoteAddr.String())
		case <-c.readTrigger:
			continue
		}
	}
	if !c.readTimer.Stop(){
		<-c.readTimer.C
	}
	return err
}

func(c *connection)fill(need int)(err error){
	if !c.lock(finalizing){
		return ErrConnClosed
	}
	defer c.unlock(finalizing)
	var n int
	var bs [][]byte
	for{
		bs =c.inputs(c.inputBarrier.bs)
		TryRead:
			n,err=ioread(c.fd,bs,c.inputBarrier.ivs)
			if err!=nil{
				break
			}
			if n==0{
				goto TryRead
			}
			c.inputAck(n)
	}
	if c.inputBuffer.Len()>=need{
		return nil
	}
	return err
}

func(c *connection)flush()error{
	if c.outputBuffer.isEmpty(){
		return nil
	}
	var bs=c.outputBuffer.GetBytes(c.outputBarrier.bs)
	var n,err=sendmsg(c.fd,bs,c.outputBarrier.ivs,false&&c.supportZeroCopy)
	if err!=nil&&err!=syscall.EAGAIN{
		  return Exception(err,"when flush")
	}
	if n>0{
		err=c.outputBuffer.Skip(n)
		c.outputBuffer.Release()
		if err!=nil{
			return Exception(err,"when flush")
		}
	}
	if c.outputBuffer.IsEmpty(){
		return nil
	}
	err =c.operator.Control(PollR2RW)
	if err!=nil{
		return Exception(err, "when flush")
	}
	return c.waitFlush()
}

func(c *connection)waitFlush()(err error){
	if c.waitTimeout == 0{
		select{
		 case err=<-c.writeTrigger:
		}
		return err
	}
	if c.writeTimer==nil{
		c.writeTimer=time.NewTimer(c.writeTimeout)
	}else{
		c.writeTimer.Reset(c.writeTimeout)
	}
	select{
	case err= <-c.writeTrigger:
		if !c.writeTimer.Stop(){
			<-c.writeTimer.C
		}
		return err
	case <-c.writeTimer.C:
		select{
		case err=<-c.writeTrigger:
			return err
		default:
		}
		c.operator.Control(PollRW2R)
		return Exception(ErrWriteTimeout, c.remoteAddr.String())
	}
}