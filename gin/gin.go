package gin

import (
	"fmt"
	"github.com/gin-gonic/gin/render"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"html/template"
	"net"
	"net/http"
	"os"
	"path"
	"regexp"
	"strings"
	"sync"
)

const defaultMultipartMemory = 32 << 20 //32MB

var (
	default404Body = []byte("404 page not found")
	default405Body = []byte("405 method not allowed")
)

var defaultPlatform string

var defaultTrustedCIDRs = []*net.IPNet{
	{
		IP: net.IP{0x0, 0x0, 0x0, 0x0},
		Mask: net.IPMask{0x0,0x0,0x0,0x0}
	},
	{
		IP:net.IP{0x0,0x0,0x0,0x0,0x0,0x0,0x0,0x0,0x0,0x0,0x0,0x0,0x0,0x0,0x0,0x0},
		Mask: net.IPMask{0x0,0x0,0x0,0x0,0x0,0x0,0x0,0x0,0x0,0x0,0x0,0x0,0x0,0x0,0x0,0x0},
	},
}

var regSafePrefix=regexp.MustCompile("[^a-zA-Z0-9/-]+")

var regRemoveRepeatedChar=regexp.MustCompile("/{2,}")


type HandlerFunc func(*Context)

type HandlersChain []HandlerFunc

func (c HandlersChain)Last()HandlerFunc{
	if length:=len(c);length>0{
		return c[length-1]
	}
	return nil
}
type RouteInfo struct {
	Method string
	Path string
	Handler string
	HandlerFunc HandlerFunc
}
type RoutesInfo []RouteInfo


const(
	PlatformGoogleAppEngine="X-Appengine-Remote-Addr"
	PlatformCloudflare="CF-Connection-IP"
)
type Engine struct {
	RouterGroup
	RedirectTrailingSlash bool
	RedirectFixedPath bool
	HandlerMethodNotAllowed bool
	ForwardedByClientIP bool
	AppEngine bool
	UseRawPath bool
	UnescapePathValues bool
	RemoveExtraSlash bool
	RemoteIPHeaders []string
	TrustedPlatform string
	MaxMultipartMemory int64
	UseH2C bool
	ContextWithFallback bool
	delims render.Delims
	secureJSONPrefix string
	HTMLRender render.HTMLRender
	FuncMap template.FuncMap
	allNoRoute HandlersChain
	allNoMethod HandlersChain
	noRoute HandlersChain
	noMethod HandlersChain
	pool sync.Pool
	trees methodTrees
	maxParams uint16
	maxSections uint16
	trustedProxies []string
	trustedCIDRs []*net.IPNet
}

var _ IRouter =  (*Engine)(nil)

func New()*Engine{
	debugPrintWARNINGNew()
	engine:=&Engine{
		RouterGroup:RouterGroup{
			Handlers:nil,
			basePath:"/",
			root: true,
		},
		FuncMap: template.FuncMap{},
		RedirectTrailingSlash: true,
		RedirectFixedPath: false,
		HandlerMethodNotAllowed: false,
		ForwardedByClientIP: true,
		RemoteIPHeaders: []string{"X-Forwarded-For","X-Real-IP"},
		TrustedPlatform: defaultPlatform,
		UseRawPath: false,
		RemoveExtraSlash: false,
		UnescapePathValues: true,
		MaxMultipartMemory: defaultMultipartMemory,
		trees: make(methodTrees,0,9),
		delims:render.Delims{Left: "{{",Right: "}}"},
		secureJSONPrefix: "while(1);",
		trustedProxies: []string{"0.0.0.0/0","::/0"},
		trustedCIDRs: defaultTrustedCIDRs,
	}
	engine.RouterGroup.engine=engine
	engine.pool.New=func()any{
		return engine.allocateContext(engine.maxParams)
	}
	return engine
}
func Default ()*Engine{
	debugPrintWARNINGDefault()
	engine:=New()
	engine.Use(Logger(),Recovery())
	return engine
}
func(engine *Engine)Handler()http.Handler{
	if !engine.UseH2C{
		return engine
	}
	h2s:=&http2.Server{}
	return h2c.NewHandler(engine,h2s)
}

func(engine *Engine)allocateContext(maxParams uint16)*Context{
	v:=make(Params,0,maxParams)
	skippedNodes:=make([]skippedNode,0,engine.maxSections)
	return &Context{engine:engine,params:&v,skippedNodes:&skippedNodes}
}

func(engine *Engine)Delims(left,right string)*Engine{
	engine.delims=render.Delims{Left: left,Right: right}
	return engine
}
func(engine *Engine)SecureJsonPrefix(prefix string)*Engine{
	engine.secureJSONPrefix=prefix
	return engine

}

func(engine *Engine)LoadHTMLGlob(pattern string){
	left:=engine.delims.Left
	right:=engine.delims.Right
	temp1:=template.Must(template.New("").Delims(left,right).Funcs(engine.FuncMap).ParseGlob(pattern))
	if IsDebugging(){
		debugPrintLoadTemplate(temp1)
		engine.HTMLRender=render.HTMLDebug{Glob: pattern,FuncMap: engine.FuncMap,Delims: engine.delims}
		return
	}
	engine.SetHTMLTemplate(temp1)
}

func(engine *Engine)LoadHTMLFiles(files ...string){
	if IsDebugging(){
		engine.HTMLRender=render.HTMLDebug{Files: files,FuncMap: engine.FuncMap,Delims: engine.delims}

	}
	temp1:=template.Must(template.New("").Delims(engine.delims.Left,engine.delims.Right).Funcs(engine.FuncMap).ParseFiles(files...))
	engine.SetHTMLTemplate(temp1)
}

func(engine *Engine)SetHTMLTemplate(temp1 *template.Template){
	if len(engine.trees)>0{
		debugPrintWARNINGSetHTMLTemplate()
	}
	engine.HTMLRender=render.HTMLProduction{Template: temp1.Funcs(engine.FuncMap)}
}

func(engine *Engine)SetFuncMap(funcMap template.FuncMap){
	engine.FuncMap=funcMap
}
func(engine *Engine)NoRoute(handlers ...HandlerFunc){
	engine.noRoute=handlers
	engine.rebuild404Handlers()
}

func(engine *Engine)NoMethod(handlers ...HandlerFunc){
	engine.noMethod=handlers
	engine.rebuild405Handlers()
}
func(engine *Engine)Use(middlerware ...HandlerFunc)IRoutes{
	engine.RouterGroup.Use(middlerware...)
	engine.rebuild404Handlers()
	engine.rebuild405Handlers()
	return engine
}
func(engine *Engine)rebuild404Handlers(){
	engine.allNoRoute=engine.combineHandlers(engine.noRoute)
}
func(engine *Engine)rebuild405Handlers(){
	engine.allNoMethod=engine.combieHandlers(engine.noMethod)
}
func(engine *Engine)addRoute(method,path string,handlers HandlersChain){
	assert1(path[0]=='/',"path muse begin with '/'")
	assert1(method!="","HTTP method can not be empty")
	assert1(len(handlers)>0,"there muse be at least one handler")
	debugPrintRoute(method,path,handlers)
	root:=engine.trees.get(method)
	if root ==nil{
		root=new(node)
		root.fullPath="/"
		engine.trees=append(engine.trees,methodTree{method:method,root:root})
	}
	root.addRoute(path,handlers)
	if paramsCount:=countParams(path);paramsCount>engine.maxParams{
		engine.maxParams=paramsCount
	}
	if sectionsCount:=countSections(path);sectionsCount>engine.maxSections{
		engine.maxSections=sectionsCount
	}

}

func(engine *Engine)Routes()(routes RoutesInfo){
	for _,tree:=range engine.trees{
		routes=iterate("",tree.method,routes,tree.root)
	}
	return routes
}
func iterate(path,method string,routes RoutesInfo,root *node)RoutesInfo{
	path +=root.path
	if len(root.handlers)>0{
		handlerFunc:=root.handlers.Last()
		routers=append(routes,RouteInfo{
			Method: method,
			Path: path,
			Handler: nameOfFunction(handlerFunc),
		    HandlerFunc: handlerFunc,
		})
	}
	for _,child:=range root.children{
		routes=iterate(path,method,routes,child)
	}
	return routes
}
func(engine *Engine)Run(addr ...string)(err error){
	defer func() {
		debugPrintError(err)
	}()

	if engine.isUnsafeTrustedProxies(){
		debugPrint("[WARNING] You trusted all proxies, this is NOT safe. We recommend you to set a value.\n" +
			"Please check https://pkg.go.dev/github.com/gin-gonic/gin#readme-don-t-trust-all-proxies for details.")
	}
	address:=resolveAddress(addr)
	debugPrint("Listening and serving HTTP on %s\n",address)
	err =http.ListenAndServe(address,engine.Handler())
	return
}
func(engine *Engine)prepareTrustedCIDRs()([]*net.IPNet,error){
	if engine.trustedProxies==nil{
		return nil,nil
	}
	cidr:=make([]*net.IPNet,0,len(engine.trustedProxies))
	for _,trustedProxy:=range engine.trustedProxies{
		if !strings.Contains(trustedProxy,"/"){
			ip:=parseIP(trustedProxy)
			if ip==nil{
				return cidr,&net.ParseError{Type: "IP address",Text: trustedProxy}
			}
			switch len(ip){
			case net.IPv4len:
				trustedProxy+="/32"
			case net.IPv6len:
				trustedProxy+="/128"
			}

		}
		_,cidrNet,err:=net.ParseCIDR(trustedProxy)
		if err!=nil{
			return cidr,err
		}
		cidr=append(cidr,cidrNet)
	}
	return cidr,nil
}

func(engine *Engine)SetTrustedProxies(trustedProxies []string)error{
	engine.trustedProxies=trustedProxies
	return  engine.parseTrustedProxies()
}
func(engine *Engine)isUnsafeTrustedProxies()bool{
	return engine.isTrustedProxy(net.ParseIP("0.0.0.0"))||engine.isTrustedProxy(net.ParseIP("::"))
}
func(engine *Engine)parseTrustedProxies()error{
        trustedCIDRs,err:=engine.prepareTrustedCIDRs()
		engine.trustedCIDRs=trustedCIDRs
		return err
}
func(engine *Engine)isTrustedProxy(ip net.IP)bool{
	if engine.trustedCIDRs==nil{
		return false
	}
	for _,cidr:=range engine.trustedCIDRs{
		if cidr.Contains(ip){
			return true
		}
	}
	return false
}

func(engine *Engine)validateHeader(header string)(clientIP string,valid bool){
	if header ==""{
		return "",false
	}
	items:=strings.Split(header,"")
	for i:=len(items)-1;i>=0;i--{
		ipStr:=strings.TrimSpace(items[i])
		ip:=net.ParseIP(ipStr)
		if ip==nil{
			break
		}
		if (i==0)||(!engine.isTrustedProxy(ip)){
			return ipStr,true
		}
	}
	return "",false
}

func parseIP(ip string)net.IP{
	parsedIP:=net.ParseIP(ip)
	if ipv4:=parsedIP.To4();ipv4!=nil{
		return ipv4
	}
	return parsedIP
}

func(engine *Engine)RunTLS(addr,certFile,keyFile string)(err error){
	debugPrint("Listening and serving HTTPS on %s\n",addr)
	defer func() {
		debugPrintError(err)
	}()
	if engine.isUnsafeTrustedProxies(){
		debugPrint("[WARNING] You trusted all proxies, this is NOT safe. We recommend you to set a value.\n" +
			"Please check https://pkg.go.dev/github.com/gin-gonic/gin#readme-don-t-trust-all-proxies for details.")
	}
	err =http.ListenAndServeTLS(addr,certFile,keyFile,engine.Handler())
	return
}

func(engine *Engine)RunUnix(file string)(err error){
	debugPrint("Listening and serving HTTP on unix:/%s",file)
	defer func() {
		debugPrintError(err)
	}()
	if engine.isUnsafeTrustedProxies(){
		debugPrint("[WARNING] You trusted all proxies, this is NOT safe. We recommend you to set a value.\n" +
			"Please check https://pkg.go.dev/github.com/gin-gonic/gin#readme-don-t-trust-all-proxies for details.")
	}
	listener,err:=net.Listen("unix",file)
	if err!=nil{
		return
	}
	defer listener.Close()
	defer os.Remove(file)
	err =http.Serve(listener,engine.Handler())
	return
}


func(engine *Engine)RunFd(fd int)(err error){
	debugPrint("Listening and serving HTTP on fd@%d",fd)
	defer func() {
		debugPrintError(err)
	}()
	if engine.isUnsafeTrustedProxies(){
		debugPrint("[WARNING] You trusted all proxies, this is NOT safe. We recommend you to set a value.\n" +
			"Please check https://pkg.go.dev/github.com/gin-gonic/gin#readme-don-t-trust-all-proxies for details.")
	}
	f :=os.NewFile(uintptr(fd),fmt.Sprintf("fd@%d",fd))
	listener ,err :=net.FileListener(f)
	if err!=nil{
		return
	}
	defer listener.Close()
	err =engine.RunListener(listener)
	return
}
func(engine *Engine)RunListener(listener net.Listener)(err error){
	debugPrint("Listening and serving HTTP on listener what's bind with address@%s",listener.Addr())
	defer func() {
		debugPrintError(err)
	}()
	if engine.isUnsafeTrustedProxies(){
		debugPrint("[WARNING] You trusted all proxies, this is NOT safe. We recommend you to set a value.\n" +
			"Please check https://pkg.go.dev/github.com/gin-gonic/gin#readme-don-t-trust-all-proxies for details.")
	}
	err =http.Serve(listener,engine.Handler())
	return
}
func(engine *Engine)ServeHTTP(w http.ResponseWriter,req *http.Request){
	c:=engine.pool.Get().(*Context)
	c.writermem.reset(w)
	c.Request=req
	c.reset()
	engine.handleHTTPRequest(c)
	engine.pool.Put(c)
}
func(engine *Engine)HandleContext(c *Context){
	oldIndexValue:=c.index
	c.reset()
	engine.handleHTTPRequest(c)
	c.index=oldIndexValue
}
func(engine *Engine)handleHTTPRequest(c *Context){
	httpMethod:=c.Request.Method
	rPath:=c.Request.URL.Path
	unescape:=false
	if engine.UseRawPath&&len(c.Request.URL.RawPath)>0{
		rPath=c.Request.URL.RawPath
		unescape=engine.UnescapePathValues
	}
	if engine.RemoveExtraSlash{
		rPath=cleanPath(rPath)
	}
	t:=engine.trees
	for  i,t1:=0,len(t);i<t1;i++{
		if t[i].mehotd!=httpMethod{
			continue
		}
		root:=t[i].root
		value:=root.getValue(rPath,c.params,c.skippedNodes,unescape)
		if value.params!=nil{
			c.Params=*value.params
		}
		if value.handlers!=nil{
			c.handlers=value.handlers
			c.fullPath=value.fullPath
			c.Next()
			c.writermem.WriteHeaderNow()
			return
		}
		if httpMethod!=http.MethodConnect &&rPath!="/"{
			if value.tsr &&engine.RedirectTrailingSlash{
				redirectTrailingSlash(c)
				return
			}
			if engine.RedirectFixedPath && redirectFixedPath(c,root,engine.RedirectFixedPath){
				return
			}
		}
		break

	}

	if engine.HandlerMethodNotAllowed{
		for _,tree:=range engine.trees{
			if tree.method ==httpMethod{
				continue
			}
			if value:=tree.root.getValue(rPath,nil,c.skippedNodes,unescape);value.handlers!=nil{
				c.handlers=engine.allNoMethod
				serverError(c,http.StatusMethodNotAllowed,default405Body)
				return
			}
		}
	}
	c.handlers=engine.allNoRoute
	serveError(c,http.StatusNotFound,default404Body)
}

var mimePlain=[]string{MIMEPlain}

func serveError(c *Context,code int,defaultMessage []byte){
	c.writermem.status=code
	c.Next()
	if c.writermem.Written(){
		return
	}
	if c.writermem.Status()==code{
		c.writermem.Header()["Content-Type"]=mimePlain
		_,err:=c.Writer.Write(defaultMessage)
		if err!=nil{
			debugPrint("cannot write message to writer during serve error: %v",err)
		}
		return
	}
	c.writermem.WriteHeaderNow()
}

func redirectTrailingSlash(c *Context){
	req:=c.Request
	p:=req.URL.Path
	//"X-Forwarded-Prefix" 是一个 HTTP 头部字段，它被代理服务器用来告诉后端服务器当前使用的 URL 前缀。例如，在反向代理中，客户端请求的 URL 实际上是代理服务器的 URL，而不是后端服务器的 URL。为了让后端服务器知道客户端请求的原始 URL，代理服务器可以在 HTTP 请求头中添加 "X-Forwarded-Prefix" 字段，指定当前正在使用的 URL 前缀。
	//GET /app1/home HTTP/1.1
	//Host: example.com
	//X-Forwarded-Proto: https
	//X-Forwarded-For: 192.0.2.1
	//X-Forwarded-Port: 443
	//X-Forwarded-Prefix: /app1
	//需要注意的是， "X-Forwarded-Prefix" 与 "X-Forwarded-For" 不同。"X-Forwarded-For" 可以用来指定客户端的 IP 地址，而 "X-Forwarded-Prefix" 用于指定代理服务器前缀。
	if prefix:=path.Clean(c.Request.Header.Get("X-Forwarded-Prefix"));prefix!="."{
		prefix=regSafePrefix.ReplaceAllString(prefix,"")
		prefix=regRemoveRepeatedChar.ReplaceAllString(prefix,"/")
		p=prefix+"/"+req.URL.Path
	}
	req.URL.Path=p+"/"
	if length:=len(p);length>1&&p[length-1]=="/"{
		req.URL.Path=p[:length-1]
	}
	redirectRequest(c)
}

func redirectFixedPath(c *Context,root *node,trailingSlash bool)bool{
	req:=c.Request
	rPath:=req.URL.Path
	if fixedPath,ok:=root.findCaseInsensitivePath(cleanPath(rPath),trailingSlash);ok{
		req.URL.Path=bytesconv.BytesToString(fixedPath)
		redirectRequest(c)
		return true
	}
	return false
}
func redirectRequest(c *Context){
	req:=c.Request
	rPath:=req.URL.Path
	rURL:=req.URL.String()
	//HTTP 301 是一种重定向状态响应码，它表示请求的资源已永久移动到Location头部给出的URL中，且后续请求的资源应该使用重定向后的URL进行访问。
	//
	//根据 [1] 和 [2] 可知，301被认为是将用户从HTTP协议升级到HTTPS协议的最佳实践之一，并且搜索引擎会更新其对该资源的链接。换句话说，如果一个网页 URL 使用301重定向到另一个 URL，那么搜索引擎将会把它对原始 URL 的 PageRank 值转移到新的 URL 上。
	//
	//需要注意的是，由于301表示永久性转移，因此浏览器、搜索引擎以及其他客户端都将缓存重定向的响应，除非显式地清除浏览器缓存或者在响应中包含 Cache-Control: no-cache。

	code:=http.StatusMovedPermanently
	if req.Method!=http.MethodGet{
		//HTTP 307是一种重定向状态响应码，表示所请求的资源已被临时移动到Location头部给出的URL中，且原始请求的方法和主体将被重复使用以执行重定向请求。
		//Location 是一个 HTTP 响应头部字段，表示被用户请求访问的资源的位置。该头部通常在重定向响应中使用，包含了重定向后的新URL地址。
		code=http.StatusTemporaryRedirect
	}
	debugPrint("redirecting request %d %s  --> %s",code,rPath,rURL )
	http.Redirect(c.Writer,req,rURL,code)
	c.writermem.WriteHeaderNow()
}