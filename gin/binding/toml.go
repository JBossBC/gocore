package binding

import (
	"bytes"
	"github.com/pelletier/go-toml/v2"
	"io"
	"net/http"
)

type tomlBinding struct {
}

func (tomlBinding) Name() string {
	return "toml"
}
func (tomlBinding) Bind(req *http.Request, obj any) error {
	return decodeToml(req.Body, obj)
}
func (tomlBinding) BindBody(body []byte, obj any) error {
	return decodeToml(bytes.NewReader(body), obj)
}
func decodeToml(r io.Reader, obj any) error {
	decoder := toml.NewDecoder(r)
	if err := decoder.Decode(obj); err != nil {
		return err
	}
	return decoder.Decode(obj)
}
