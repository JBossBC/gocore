package binding

import (
	"errors"
	"google.golang.org/protobuf/proto"
	"io"
	"net/http"
)

type protobufBinding struct{}

func (protobufBinding) Name() string {
	return "protobuf"
}
func (b protobufBinding) Bind(req *http.Request, obj any) error {
	buf, err := io.ReadAll(req.Body)
	if err != nil {
		return err
	}
	return b.BindBody(buf, obj)
}
func (protobufBinding) BindBody(body []byte, obj any) error {
	msg, ok := obj.(proto.Message)
	if !ok {
		return errors.New("obj is not ProtoMessage")
	}
	if err := proto.Unmarshal(body, msg); err != nil {
		return err
	}
	return nil
}
