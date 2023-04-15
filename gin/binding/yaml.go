package binding

import (
	"bytes"
	"gopkg.in/yaml.v2"
	"io"
	"net/http"
)

type yamlBinding struct{}

func (yamlBinding) Name() string {
	return "yaml"
}
func (yamlBinding) Bind(req *http.Request, obj any) error {
	return decodeYAML(req.Body, obj)
}
func (yamlBinding) BindBody(body []byte, obj any) error {
	return decodeYAML(bytes.NewReader(body), obj)
}
func decodeYAML(r io.Reader, obj any) error {
	decoder := yaml.NewDecoder(r)
	if err := decoder.Decode(obj); err != nil {
		return err
	}
	return validate(obj)
}
