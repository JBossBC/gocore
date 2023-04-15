package binding

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

var EnableDecoderUseNumber = false

var EnableDecoderDisallowUnknownFields = false

type jsonnBinding struct {
}

func (jsonnBinding) Name() string {
	return "json"
}
func (jsonnBinding) Bind(req *http.Request, obj any) error {
	if req == nil || req.Body == nil {
		return errors.New("invalid request")
	}
	return decodeJSON(req.Body, obj)
}
func (jsonnBinding) BindBody(body []byte, obj any) error {
	return decodeJSON(bytes.NewReader(body), obj)
}
func decodeJSON(r io.Reader, obj any) error {
	decoder := json.NewDecoder(r)
	if EnableDecoderUseNumber {
		decoder.UseNumber()
	}
	if EnableDecoderDisallowUnknownFields {
		decoder.DisallowUnknownFields()
	}
	if err := decoder.Decode(obj); err != nil {
		return err
	}
	return validate(obj)
}
