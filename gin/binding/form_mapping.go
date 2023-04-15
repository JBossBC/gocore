package binding

import (
	"errors"
	"fmt"
	"reflect"
)

var (
	errUnknownType           = errors.New("unknown type")
	ErrConvertMapStringSlice = errors.New("can not convert to map slices of strings")
	ErrConvertToMapString    = errors.New("can not convert to map of strings")
)

func mapURI(ptr any, m map[string][]string) error {
	return mapFormByTag(ptr, m, "uri")
}
func mapForm(ptr any, form map[string][]string) error {
	return mapFormByTag(ptr, form, "form")
}
func mapFormWithTag(ptr any, form map[string][]string, tag string) error {
	return mapFormByTag(ptr, form, tag)
}

var emptyField = reflect.StructField{}

func mapFormByTag(ptr any, form map[string][]string, tag string) error {
	ptrVal := reflect.ValueOf(ptr)
	var pointed any
	if ptrVal.Kind() == reflect.Ptr {
		ptrVal = ptrVal.Elem()
		pointed = ptrVal.Interface()
	}
	if ptrVal.Kind() == reflect.Map && ptrVal.Type().Key().Kind() == reflect.String {
		if pointed != nil {
			ptr = pointed
		}
		return setFormMap(ptr, form)
	}

	return mappingByPtr(ptr, formSource(form), tag)
}

type setter interface {
	TrySet(value reflect.Value, field reflect.StructField, key string, opt setOptions) (isSet bool, err error)
}

type formSource map[string][]string

var _ setter = formSource(nil)

func (form formSource) TrySet(value reflect.Value, field reflect.StructField, tagValue string, opt setOptions) (isSet bool, err error) {
	return setByForm(value, field, form, tagValue, opt)
}
func mappingByPtr(ptr any, setter setter, tag string) error {
	_, err := mapping(reflect.ValueOf(ptr), emptyField, setter, tag)
	return err
}

//the emptyField ?????/
func mapping(value reflect.Value, field reflect.StructField, setter setter, tag string) (bool, error) {
	if field.Tag.Get(tag) == "-" {
		return false, nil
	}
	vKind := value.Kind()
	if vKind == reflect.Ptr {
		var isNew bool
		vPtr := value
		if value.IsNil() {
			isNew = true
			vPtr = reflect.New(value.Type().Elem())
		}
		isSet, err := mapping(vPtr.Elem(), field, setter, tag)
		if err != nil {
			return false, err
		}
		if isNew && isSet {
			value.Set(vPtr)
		}
		return isSet, nil
	}
	//在 Go 语言中，结构体可以包含其他的结构体或基本数据类型作为其成员变量。当一个结构体成员变量被声明为匿名字段时，其成员名称会被省略，只保留其数据类型。这种方式可以简化结构体的定义，使其更加紧凑和易于阅读。
	if vKind != reflect.Struct || !field.Anonymous {
		ok, err := tryToSetValue(value, field, setter, tag)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}
	if vKind == reflect.Struct {
		tValue := value.Type()
		var isSet bool
		for i := 0; i < value.NumField(); i++ {
			sf := tValue.Field(i)
			//是否属于本包并且是否是匿名字段
			if sf.PkgPath != "" && !sf.Anonymous {
				continue
			}
			ok, err := mapping(value.Field(i), sf, setter, tag)
			if err != nil {
				return false, err
			}
			isSet = isSet || ok
		}
		return isSet, nil
	}
	return false, nil
}

type setOptions struct {
	isDefaultExists bool
	defaultValue    string
}

func tryToSetValue(value reflect.Value, field reflect.StructField, setter setter, tag string) (bool, error) {
	var tagValue string
	var setOpt setOptions
	tagValue = field.Tag.Get(tag)
	tagValue, opts := head(tagValue, ",")
	if tagValue == "" {
		tagValue = field.Name
	}
	if tagValue == "" {
		return false, nil
	}
	var opt string
	for len(opts) > 0 {
		opt, opts = head(opts, ",")
		if k, v := head(opt, "="); k == "default" {
			setOpt.isDefaultExists = true
			setOpt.defaultValue = v
		}
	}
	return setter.TrySet(value, field, tagValue, setOpt)
}
func setByForm(value reflect.Value, field reflect.StructField, form map[string][]string, tagValue string, opt setOptions) (isSet bool, err error) {
	vs, ok := form[tagValue]
	if !ok && !opt.isDefaultExists {
		return false, nil
	}
	switch value.Kind() {
	case reflect.Slice:
		if !ok {
			vs = []string{opt.defaultValue}
		}
		return true, setSlice(vs, value, field)
	case reflect.Array:
		if !ok {
			vs = []string{opt.defaultValue}
		}
		if len(vs) != value.Len() {
			return false, fmt.Errorf("%q is not valid value for %s", vs, value.Type().String())
		}
		return true, setArray(vs, value, field)
	default:
		var val string
		if !ok {
			val = opt.defaultValue
		}
		if len(vs) > 0 {
			val = vs[0]
		}
		return true, setWithProperType(val, value, field)
	}
}
