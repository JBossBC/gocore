package binding

import "io/ioutil"

type uriBinding struct{}

func (uriBinding) Name() string {
	return "uri"
}
func (uriBinding) BindUri(m map[string][]string, obj any) error {
	ioutil.NopCloser()
	if err := mapURI(obj, m); err != nil {
		return err
	}
	return validate(obj)
}
