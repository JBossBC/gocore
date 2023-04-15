package binding

import "net/http"

const defaultMemory = 32 << 20

type formBinding struct {
}
type formPostBinding struct {
}
type formMultipartBinding struct {
}

func (formBinding) Name() string {
	return "form"
}
func (formBinding) Bind(req *http.Request, obj any) error {
	if err := req.ParseForm(); err != nil {
		return err
	}
	if err := req.ParseMultipartForm(defaultMemory); err != nil {
		return err
	}
	if err := mapForm(obj, req.Form); err != nil {
		return err
	}
	return validate(obj)
}
func (formPostBinding) Name() string {
	return "form-urlencoded"
}
func (formPostBinding) Bind(req *http.Request, obj any) error {
	if err := req.ParseForm(); err != nil {
		return err
	}
	if err := mapForm(obj, req.PostForm); err != nil {
		return err
	}
	return validate(obj)
}
func (formMultipartBinding) Name() string {
	return "multipart/form-data"
}

//ParseMultipartForm 和 ParseForm 都是 Go 语言标准库中 net/http 包提供的方法，用于解析 HTTP 请求中提交的表单数据。
//
//ParseMultipartForm 用于解析 multipart/form-data 类型的请求体，整个请求体都会被解析到内存中，并且可以通过调用 FormValue 或 FormFile 方法获取表单参数或上传的文件对象。如果上传的文件大小超过了指定的 maxMemory 大小，则会将超出部分写入到临时文件中。
//
//ParseForm 则适用于解析其他类型表单（如 application/x-www-form-urlencoded）中的参数，并将所有的参数存储在 r.Form 中。对于 POST、PUT 或 PATCH 请求，在调用 ParseForm 方法之前需要先调用 r.ParseMultipartForm 方法。在解析请求体部分时，如果表单参数数量很多或者上传的文件比较大，不会像 ParseMultipartForm 那样将整个请求体解析到内存中，而是采用流式处理的方式，逐步读取请求体并解析其中的表单参数。
//
//因此，两种方法的区别在于它们处理表单数据的方式不同，针对的是不同的请求类型和场景。

func (formMultipartBinding) Bind(req *http.Request, obj any) error {
	if err := req.ParseMultipartForm(defaultMemory); err != nil {
		return err
	}
	if err := mappingByPtr(obj, (*multipartRequest)(req), "form"); err != nil {
		return err
	}
	return validate(obj)
}
