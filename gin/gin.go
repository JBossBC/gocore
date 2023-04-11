package gin

import (
	"github.com/gin-gonic/gin/render"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"html/template"
	"net"
	"net/http"
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