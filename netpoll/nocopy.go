package netpoll

import "io"

type Reader interface {
	Next(n int) (p []byte, err error)
	Peek(n int) (buf []byte, err error)
	Skip(n int) (err error)
	Until(delim byte) (line []byte, err error)
	ReadString(n int) (s string, err error)
	ReadBinary(n int) (p []byte, err error)
	ReadByte() (b byte, err error)
	Slice(n int) (r Reader, err error)
	Release() (err error)
	Len() (length int)
}
type Writer interface {
	Malloc(n int) (buf []byte, err error)
	WriteString(s string) (n int, err error)
	WriteBinary(b []byte) (n int, err error)
	WriteByte(b byte) (err error)
	WriteDirect(p []byte, remainCap int) error
	MallocAck(n int) (err error)
	Append(w Writer) (err error)
	Flush() (err error)
	MallocLen() (length int)
}

type ReadWriter interface {
	Reader
	Writer
}

func NewReader(r io.Reader) Reader {
	return newZCReader(r)
}
func NewWriter(w io.Writer) Writer {
	return newZCWriter(w)
}
func NewReadWriter(rw io.ReadWriter) ReadWriter {
	return &zcReaderWriter{
		zcReader: newZCReader(rw),
		zcWriter: newZCWriter(rw),
	}
}

// NewIOReader convert Reader to io.Reader
func NewIOReader(r Reader) io.Reader {
	if reader, ok := r.(io.Reader); ok {
		return reader
	}
	return newIOReader(r)
}

// NewIOWriter convert Writer to io.Writer
func NewIOWriter(w Writer) io.Writer {
	if writer, ok := w.(io.Writer); ok {
		return writer
	}
	return newIOWriter(w)
}

// NewIOReadWriter convert ReadWriter to io.ReadWriter
func NewIOReadWriter(rw ReadWriter) io.ReadWriter {
	if rwer, ok := rw.(io.ReadWriter); ok {
		return rwer
	}
	return &ioReadWriter{
		ioReader: newIOReader(rw),
		ioWriter: newIOWriter(rw),
	}
}

const (
	block1k = 1 * 1024
	block2k = 2 * 1024
	block4k = 4 * 1024
	block8k = 8 * 1024
)

const pagesize = block8k
