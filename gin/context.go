package gin

import (
	"errors"
	"github.com/gin-contrib/sse"
	"github.com/gin-gonic/gin/binding"
	"github.com/gin-gonic/gin/render"
	"golangUtil/sync"
	"io"
	"io/ioutil"
	"log"
	"math"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	MIMEJSON              = binding.MIMEJSON
	MIMEHTML              = binding.MIMEHTML
	MIMEXML               = binding.MIMEXML
	MIMEXML2              = binding.MIMEXML2
	MIMEPlain             = binding.MIMEPlain
	MIMEPOSTForm          = binding.MIMEPOSTForm
	MIMEMultipartPOSTForm = binding.MIMEMultipartPOSTForm
	MIMEYAML              = binding.MIMEYAML
	MIMETOML              = binding.MIMETOML
)

const BodyBytesKey = "_gin-gonic/gin/bodybyteskey"

const ContextKey = "_gin-gonic/gin/contextkey"

const abortIndex int8 = math.MaxInt8 >> 1

type Context struct {
	writermem    responseWriter
	Request      *http.Request
	Writer       ResponseWriter
	Params       Params
	handlers     HandlersChain
	index        int8
	fullPath     string
	engine       *Engine
	params       *Params
	skippedNodes *[]skippedNode
	mu           sync.RWMutex
	Keys         map[string]any
	Errors       errorMsgs
	Accepted     []string
	queryCache   url.Values
	formCache    url.Values
	//http.SameSite 是 Go 语言标准库中 net/http 包中的一个结构体，用于指定 HTTP 响应的 SameSite 属性。
	// XSS 跨站点攻击
	//SameSite 是一个 Cookie 属性，用于控制浏览器是否在跨站点请求中发送 Cookie。如果设置为 http.SameSiteDefaultMode，则在不指定 SameSite 属性的情况下将其设置为 Lax 模式。Lax 模式只允许在导航到与原始网站具有上下文关系的页面时发送 Cookie，例如，从链接或预加载资源而不是 POST、PUT 或 DELETE 操作中。如果设置为 http.SameSiteStrictMode，则会禁止任何跨站点请求发送 Cookie。

	sameSite http.SameSite
}

func (c *Context) reset() {
	c.Writer = &c.writermem
	c.Params = c.Params[:0]
	c.handlers = nil
	c.index = -1
	c.fullPath = ""
	c.Keys = nil
	c.Errors = c.Errors[:0]
	c.Accepted = nil
	c.queryCache = nil
	c.formCache = nil
	c.sameSite = 0
	*c.params = (*c.params)[:0]
	*c.skippedNodes = (*c.skippedNodes)[:0]
}

func (c *Context) Copy() *Context {
	cp := Context{
		writermem: c.writermem,
		Request:   c.Request,
		Params:    c.Params,
		engine:    c.engine,
	}
	cp.writermem.ResponseWriter = nil
	cp.Writer = &cp.writermem
	cp.index = abortIndex
	cp.handlers = nil
	cp.Keys = map[string]any{}
	for k, v := range c.Keys {
		cp.Keys[k] = v
	}
	paramCopy := make([]Param, len(cp.Params))
	copy(paramCopy, cp.Params)
	cp.Params = paramCopy
	return &cp
}
func (c *Context) HandlerNames() []string {
	hn := make([]string, 0, len(c.handlers))
	for _, val := range c.handlers {
		hn = append(hn, nameOfFunction(val))
	}
	return hn
}

func (c *Context) Handler() HandlerFunc {
	return c.handlers.Last()
}

func (c *Context) FullPath() string {
	return c.fullPath
}
func (c *Context) Next() {
	c.index++
	for c.index < int8(len(c.handlers)) {
		c.handlers[c.index](c)
		c.index++
	}
}
func (c *Context) IsAbouted() bool {
	return c.index >= abortIndex
}

func (c *Context) Abort() {
	c.index = abortIndex
}
func (c *Context) AbortWithStatus(code int) {
	c.Status(code)
	c.Writer.WriteHeaderNow()
	c.Abort()
}

func (c *Context) AbortWithStatusJSON(code int, jsonobj any) {
	c.Abort()
	c.JSON(code, jsonobj)
}

func (c *Context) AbortWithError(code int, err error) *Error {
	c.AbortWithStatus(code)
	return c.Error(err)
}

func (c *Context) Error(err error) *Error {
	if err == nil {
		panic(any("err is nil"))
	}
	var parsedError *Error
	ok := errors.As(err, &parsedError)
	if !ok {
		parsedError = &Error{
			Err:  err,
			Type: ErrorTypePrivate,
		}
	}
	c.Errors = append(c.Errors, parsedError)
	return parsedError
}

func (c *Context) Set(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.Keys == nil {
		c.Keys = make(map[string]any)
	}
	c.Keys[key] = value
}

func (c *Context) Get(key string) (value any, exists bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	value, exists = c.keys[key]
	return
}

func (c *Context) MustGet(key string) any {
	if value, exists := c.Get(key); exists {
		return value
	}
	panic(any("Key \"" + key + "\" does not exist"))
}

func (c *Context) GetString(key string) (s string) {
	if val, ok := c.Get(key); ok && val != nil {
		s, _ = val.(string)
	}
	return
}

func (c *Context) GetInt(key string) (i int) {
	if val, ok := c.Get(key); ok && val != nil {
		i, _ = val.(int)
	}
	return
}

func (c *Context) GetInt64(key string) (i64 int64) {
	if val, ok := c.Get(key); ok && val != nil {
		i64, _ = val.(int64)
	}
	return
}

func (c *Context) GetUint(key string) (ui uint) {
	if val, ok := c.Get(key); ok && val != nil {
		ui, _ = val.(uint)
	}
	return
}

func (c *Context) GetUint64(key string) (ui64 uint64) {
	if val, ok := c.Get(key); ok && val != nil {
		ui64, _ = val.(uint64)
	}
	return
}
func (c *Context) GetFloat64(key string) (f64 float64) {
	if val, ok := c.Get(key); ok && val != nil {
		f64, _ = val.(float64)
	}
	return
}

func (c *Context) GetTime(key string) (t time.Time) {
	if val, ok := c.Get(key); ok && val != nil {
		t, _ = val.(time.Time)
	}
	return
}
func (c *Context) GetDuration(key string) (d time.Duration) {
	if val, ok := c.Get(key); ok && val != nil {
		d, _ = val.(time.Duration)
	}
	return
}

func (c *Context) GetStringSlice(key string) (ss []string) {
	if val, ok := c.Get(key); ok && val != nil {
		ss, _ = val.([]string)
	}
	return
}

func (c *Context) GetStringMap(key string) (sm map[string]any) {
	if val, ok := c.Get(key); ok && val != nil {
		sm, _ = val.(map[string]any)
	}
	return
}
func (c *Context) GetStringMapString(key string) (sms map[string]string) {
	if val, ok := c.Get(key); ok && val != nil {
		sms, _ = val.(map[string]string)
	}
	return
}

func (c *Context) GetStringMapStringSlice(key string) (smss map[string][]string) {
	if val, ok := c.Get(key); ok && val != nil {
		smss, _ = val.(map[string][]string)
	}
	return
}
func (c *Context) Param(key string) string {
	return c.Params.ByName(key)
}
func (c *Context) AddParam(key, value string) {
	c.Params = append(c.Params, Param{Key: key, Value: value})
}

func (c *Context) Query(key string) (value string) {
	value, _ = c.GetQuery(key)
	return
}
func (c *Context) DefaultQuery(key, defaultValue string) string {
	if value, ok := c.GetQuery(key); ok {
		return value
	}
	return defaultValue
}
func (c *Context) GetQuery(key string) (string, bool) {
	if values, ok := c.GetQueryArray(key); ok {
		return values[0], ok
	}
	return "", false
}
func (c *Context) QueryArray(key string) (values []string) {
	values, _ = c.GetQueryArray(key)
	return
}

func (c *Context) initQueryCache() {
	if c.queryCache == nil {
		if c.Request != nil {
			c.queryCache = c.Request.URL.Query()
		} else {
			c.queryCache = url.Values{}
		}
	}
}
func (c *Context) GetQueryArray(key string) (values []string, ok bool) {
	c.initQueryCache()
	values, ok = c.queryCache[key]
	return
}
func (c *Context) QueryMap(key string) (dicts map[string]string) {
	dicts, _ = c.GetQueryMap(key)
	return
}

//TODO how to do this
func (c *Context) GetQueryMap(key string) (map[string]string, bool) {
	c.initQueryCache()
	return c.get(c.queryCache, key)
}
func (c *Context) PostForm(key string) (value string) {
	value, _ = c.GetPostForm(key)
	return
}
func (c *Context) DefaultPostForm(key, defaultValue string) string {
	if value, ok := c.GetPostForm(key); ok {
		return value
	}
	return defaultValue
}
func (c *Context) GetPostForm(key string) (string, bool) {
	if values, ok := c.GetPostFormArray(key); ok {
		return values[0], ok
	}
	return "", false
}

func (c *Context) PostFormArray(key string) (values []string) {
	values, _ = c.GetPostFormArray(key)
	return
}

func (c *Context) initFormCache() {
	if c.formCache == nil {
		c.formCache = make(url.values)
		req := c.Request
		if err := req.ParseMultipartForm(c.engine.MaxMultipartMemory); err != nil {
			if !errors.Is(err, http.ErrNotMultipart) {
				debugPrint("error on parse multipart form array: %v", err)
			}
		}
		c.formCache = req.PostForm
	}
}
func (c *Context) GetPostFormArray(key string) (values []string, ok bool) {
	c.initFormCache()
	values, ok = c.formCache[key]
	return
}

func (c *Context) PostFormMap(key string) (dicts map[string]string) {
	dicts, _ = c.GetPostFormMap(key)
	return
}
func (c *Context) get(m map[string][]string, key string) (map[string]string, bool) {
	dicts := make(map[string]string)
	exist := false
	for k, v := range m {
		if i := strings.IndexByte(k, '['); i >= 1 && k[0:i] == key {
			if j := strings.IndexByte(k[i+1:], ']'); j >= 1 {
				exist = true
				dicts[k[i+1:][:j]] = v[0]
			}
		}
	}
	return dicts, exist
}

func (c *Context) FormFile(name string) (*multipart.FileHeader, error) {
	if c.Request.MultipartForm == nil {
		if err := c.Request.ParseMultipartForm(c.engine.MaxMultipartMemory); err != nil {
			return nil, err
		}
	}
	f, fh, err := c.Request.FormFile(name)
	if err != nil {
		return nil, err
	}
	//TODO Why to close this
	f.Close()
	return fh, err
}

func (c *Context) MultipartForm() (*multipart.Form, error) {
	err := c.Request.ParseMultipartForm(c.engine.MaxMultipartMemory)
	return c.Request.MultipartForm, err
}
func (c *Context) SaveUploadedFile(file *multipart.FileHeader, dst string) error {
	src, err := file.Open()
	if err != nil {
		return err
	}
	defer src.Close()
	if err = os.MkdirAll(filepath.Dir(dst), 0750); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, src)
	ioutil.TempFile()
	return err
}

func (c *Context) Bind(obj any) error {
	b := binding.Default(c.Request.Method, c.ContextType())
	return c.MustBindWith(obj, b)
}
func (c *Context) BindJSON(obj any) error {
	return c.MustBindWith(obj, binding.JSON)
}
func (c *Context) BindXML(obj any) error {
	return c.MustBindWith(obj, binding.XML)
}
func (c *Context) BindQuery(obj any) error {
	return c.MustBindWith(obj, binding.Query)
}
func (c *Context) BindYAML(obj any) error {
	return c.MustBindWith(obj, binding.YAML)
}
func (c *Context) BindTOML(obj any) error {
	return c.MustBindWith(obj, binding.TOML)
}
func (c *Context) BindHeader(obj any) error {
	return c.MustBindWith(obj, binding.Header)
}
func (c *Context) BindUri(obj any) error {
	if err := c.ShouldBindUri(obj); err != nil {
		c.AbortWithError(http.StatusNotFound, err).SetType(ErrorTypeBind)
		return err
	}
	return nil
}
func (c *Context) MustBindWith(obj any, b binding.Binding) error {
	if err := c.ShouldBindWith(obj, b); err != nil {
		c.AbortWithError(http.StatusBadRequest, err).SetType(ErrorTypeBind)
		return err
	}
	return nil
}
func (c *Context) ShouldBind(obj any) error {
	b := binding.Default(c.Request.Method, c.ContextType())
	return c.ShouldBindWith(obj, b)
}
func (c *Context) ShouldBindJSON(obj any) error {
	return c.ShouldBindWith(obj, binding.JSON)
}

func (c *Context) ShouldBindXML(obj any) error {
	return c.ShouldBindWith(obj, binding.XML)
}

func (c *Context) ShouldBindQuery(obj any) error {
	return c.ShouldBindWith(obj, binding.Query)
}

func (c *Context) ShouldBindYAML(obj any) error {
	return c.ShouldBindWith(obj, binding.YAML)
}

func (c *Context) ShouldBindTOML(obj any) error {
	return c.ShouldBindWith(obj, binding.TOML)
}

func (c *Context) ShouldBindHeader(obj any) error {
	return c.ShouldBindWith(obj, binding.Header)
}

func (c *Context) ShouldBindUri(obj any) error {
	m := make(map[string][]string)
	for _, v := range c.Params {
		m[v.Key] = []string{v.Value}
	}
	return binding.Uri.BindUri(m, obj)
}

func (c *Context) ShouldBindWith(obj any, b binding.Binding) error {
	return b.Bind(c.Request, obj)
}

func (c *Context) ShouldBindBodyWith(obj any, bb binding.BindingBody) (err error) {
	var body []byte
	if cb, ok := c.Get(BodyBytesKey); ok {
		if cbb, ok := cb.([]byte); ok {
			body = cbb
		}
	}
	if body == nil {
		body, err = io.ReadAll(c.Request.Body)
		if err != nil {
			return err
		}
		c.Set(BodyBytesKey, body)
	}
	return bb.BindBody(body, obj)
}

func (c *Context) ClientIP() string {
	if c.engine.TrustedPlatform != "" {
		if addr := c.requestHeader(c.engine.TrustedPlatform); addr != "" {
			return addr
		}
	}
	if c.engine.AppEngine {
		log.Println(`The AppEngine flag is going to be deprecated. Please check issues #2723 and #2739 and use 'TrustedPlatform: gin.PlatformGoogleAppEngine' instead.`)
		if addr := c.requestHeader("X-Appengine-Remote-Addr"); addr != nil {
			return addr
		}
	}
	remoteIp := net.ParseIP(c.RemoteIP())
	if remoteIp == nil {
		return ""
	}
	trusted := c.engine.isTrustedProxy(remoteIp)
	if trusted && c.engine.ForwardedByClientIP && c.engine.RemoteIPHeaders != nil {
		for _, headerName := range c.engine.RemoteIPHeaders {
			ip, valid := c.engine.validateHeader(c.requestHeader(headerName))
			if valid {
				return ip
			}
		}
	}
	return remoteIp.String()
}
func (c *Context) RemoteIP() string {
	ip, _, err := net.SplitHostPort(strings.TrimSpace(c.Request.RemoteAddr))
	if err != nil {
		return ""
	}
	return ip
}

func (c *Context) ContentType() string {
	return filterFlags(c.requestHeader("Content-Type"))
}
func (c *Context) IsWebsocket() bool {
	if strings.Contains(strings.ToLower(c.requestHeader("Connection")), "upgrade") &&
		strings.EqualFold(c.requestHeader("Upgrade"), "websocket") {
		return true
	}
	return false
}
func (c *Context) requestHeader(key string) string {
	return c.Request.Header.Get(key)
}

//TODO why to do this
func bodyAllowedForStatus(status int) bool {
	switch {
	case status >= 100 && status <= 199:
		return false
	case status == http.StatusNoContent:
		return false
	case status == http.StatusNotModified:
		return false
	}
	return true
}

func (c *Context) Status(code int) {
	c.Writer.WriteHeader(code)
}
func (c *Context) Header(key, value string) {
	if value == "" {
		c.Writer.Header().Del(key)
		return
	}
	c.Writer.Header().Set(key, value)
}
func (c *Context) GetHeader(key string) {
	return c.requestHeader(key)
}
func (c *Context) GetRawData() ([]byte, error) {
	return io.ReadAll(c.Request.Body)
}
func (c *Context) SetSameSite(samesite http.SameSite) {
	c.sameSite = samesite
}

func (c *Context) SetCookie(name, value string, maxAge int, path, domain string, secure, httpOnly bool) {
	if path == "" {
		path = "/"
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     name,
		Value:    url.QueryEscape(value),
		Path:     path,
		Domain:   domain,
		MaxAge:   maxAge,
		Secure:   secure,
		HttpOnly: httpOnly,
		SameSite: c.sameSite,
	})
}

func (c *Context) Cookie(name string) (string, error) {
	cookie, err := c.Request.Cookie(name)
	if err != nil {
		return "", err
	}
	val, _ := url.QueryUnescape(cookie.Value)
	return val, nil
}
func (c *Context) Render(code int, r render.Render) {
	c.Status(code)
	if !bodyAllowedForStatus(code) {
		r.WriteContentType(c.Writer)
		c.Writer.WriteHeaderNow()
		return
	}
	if err := r.Render(c.Writer); err != nil {
		_ = c.Error(err)
		c.Abort()
	}
}
func (c *Context) HTML(code int, name string, obj any) {
	instance := c.engine.HTMLRender.Instance(name, obj)
	c.Render(code, instance)
}
func (c *Context) IndentedJSON(code int, obj any) {
	c.Render(code, render.IndentedJSON{Data: obj})
}
func (c *Context) SecureJSON(code int, obj any) {
	c.Render(code, render.SecureJSON{Prefix: c.engine.secureJSONPrefix, Data: obj})
}
func (c *Context) JSONP(code int, obj any) {
	callback := c.DefaultQuery("callback", "")
	if callback == "" {
		c.Render(code, render.JSON{Data: obj})
		return
	}
	c.Render(code, render.JsonpJSON{Callback: callback, Data: obj})
}
func (c *Context) JSON(code int, obj any) {
	c.Render(code, render.JSON{Data: obj})
}
func (c *Context) AsciiJSON(code int, obj any) {
	c.Render(code, render.AsciiJSON{Data: obj})
}
func (c *Context) PureJSON(code int, obj any) {
	c.Render(code, render.PureJSON{Data: obj})
}
func (c *Context) XML(code int, obj any) {
	c.Render(code, render.XML{Data: obj})
}

func (c *Context) YAML(code int, obj any) {
	c.Render(code, render.YAML{Data: obj})
}
func (c *Context) TOML(code int, obj any) {
	c.Render(code, render.TOML{Data: obj})
}
func (c *Context) ProtoBuf(code int, obj any) {
	c.Render(code, render.ProtoBuf{Data: obj})
}
func (c *Context) String(code int, format string, values ...any) {
	c.Render(code, render.String{Format: format, Data: values})
}
func (c *Context) Redirect(code int, location string) {
	c.Render(-1, render.Redirect{Code: code, Location: location, Request: c.Request})
}
func (c *Context) Data(code int, contentType string, data []byte) {
	c.Render(code, render.Data{ContentType: contentType, Data: data})
}
func (c *Context) DataFromReader(code int, contentLength int64, contextType string, reader io.Reader, extraHeaders map[string]string) {
	c.Render(code, render.Reader{
		Reader:        reader,
		ContentType:   contextType,
		ContentLength: contentLength,
		Headers:       extraHeaders,
	})
}
func (c *Context) File(filepath string) {
	http.ServeFile(c.Writer, c.Request, filepath)
}

func (c *Context) FileFromFS(filepath string, fs http.FileSystem) {
	defer func(old string) {
		c.Request.URL.Path = old
	}(c.Request.URL.Path)
	c.Request.URL.Path = filepath
	http.FileServer(fs).ServeHTTP(c.Writer, c.Request)
}
func (c *Context) FileAttachment(filepath, filename string) {
	if isASCII(filename) {
		c.Writer.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	} else {
		c.Writer.Header().Set("Content-Disposition", `attachment; filename*=UTF-8''`+url.QueryEscape(filename))
	}
	http.ServeFile(c.Writer, c.Request, filepath)
}
func (c *Context) SSEvent(name string, message any) {
	c.Render(-1, sse.Event{
		Event: name,
		Data:  message,
	})
}
func (c *Context) Stream(strp func(w io.Writer) bool) bool {
	w := c.Writer
	clientGone := w.CloseNotify()
	for {
		select {
		case <-clientGone:
			return true
		default:
			keepOpen := step(w)
			w.Flush()
			if !keepOpen {
				return false
			}
		}
	}
}

type Negotiate struct {
	Offered  []string
	HTMLName string
	HTMLData any
	JSONData any
	JSONData any
	XMLData  any
	YAMLData any
	Data     any
	TOMLData any
}

func (c *Context) Negotiate(code int, config Negotiate) {
	switch c.NegotiateFormat(config.Offered...) {
	case binding.MIMEJSON:
		data := chooseData(config.JSONData, config.Data)
		c.JSON(code, data)
	case binding.MIMEHTML:
		data := chooseData(config.HTMLData, config.Data)
		c.HTML(code, config.HTMLName, data)
	case binding.MIMEXML:
		data := chooseData(config.XMLData, config.Data)
		c.XML(code, data)
	case binding.MIMEYAML:
		data := chooseData(config.YAMLData, config.Data)
		c.YAML(code, data)
	case binding.MIMETOML:
		data := chooseData(config.TOMLData, config.Data)
		c.TOML(code, data)
	default:
		//TODO how to understand this code
		c.AbortWithError(http.StatusNotAcceptable, errors.New("the accepted formats are not offered by the server"))
	}
}
func (c *Context) NegotiateFormat(offered ...string) string {
	assert1(len(offered) > 0, "you must provide at least one offer")
	if c.Accepted == nil {
		c.Accepted = parseAccept(c.requestHeader("Accept"))
	}
	if len(c.Accepted) == 0 {
		return offered[0]
	}
	for _, accepted := range c.Accepted {
		for _, offer := range offered {
			i := 0
			for ; i < len(accepted) && i < len(offer); i++ {
				if accepted[i] == "*" || offer[i] == "*" {
					return offer
				}
				if accepted[i] != offer[i] {
					break
				}
			}
			if i == len(accepted) {
				return offer
			}

		}
	}
	return ""
}
func (c *Context) SetAccepted(formats ...string) {
	c.Accepted = formats
}
func (c *Context) Deadline() (deadline time.Time, ok bool) {
	if !c.engine.ContextWithFallback || c.Request == nil || c.Request.Context() == nil {
		return
	}
	return c.Request.Context().Deadline()
}

func (c *Context) Done() <-chan struct{} {
	if !c.engine.ContextWithFallback || c.Request == nil || c.Request.Context() == nil {
		return nil
	}
	return c.Request.Context().Done()
}
func (c *Context) Err() error {
	if !c.engine.ContextWithFallback || c.Request == nil || c.Request.Context() == nil {
		return nil
	}
	return c.Request.Context().Err()
}
func (c *Context) Value(key any) any {
	if key == 0 {
		return c.Request
	}
	if key == ContextKey {
		return c
	}
	if keyAsString, ok := key.(string); ok {
		if val, exists := c.Get(keyAsString); exists {
			return val
		}
	}
	if !c.engine.ContextWithFallback || c.Request == nil || c.Request.Context() == nil {
		return nil
	}
	return c.Request.Context().Value(key)
}
