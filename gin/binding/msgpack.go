package binding

import (
	"bytes"
	"github.com/ugorji/go/codec"
	"io"
	"net/http"
)

type msgpackBinding struct {
}

func (msgpackBinding) Name() string {
	return "msgpack"
}
func (msgpackBinding) Bind(req *http.Request, obj any) error {
	return decodeMsgPack(req.Body, obj)
}
func (msgpackBinding) BindBody(body []byte, obj any) error {
	return decodeMsgPack(bytes.NewReader(body), obj)
}
func decodeMsgPack(r io.Reader, obj any) error {
	cdc := new(codec.MsgpackHandle)
	if err := codec.NewDecoder(r, cdc).Decode(&obj); err != nil {
		return err
	}
	return validate(obj)
}
