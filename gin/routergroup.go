package gin

import (
	"net/http"
	"path"
	"regexp"
	"strings"
)

var (
	regEnLetter = regexp.MustCompile("^[A-Z]+$")
	anyMethods  = []string{
		http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodHead, http.MethodOptions, http.MethodDelete, http.MethodConnect, http.MethodTrace,
	}
)

type IRouter interface {
	IRoutes
	Group(string ...HandlerFunc) *RouterGroup
}

type IRoutes interface {
	Use(...HandlerFunc) IRoutes
	Handle(string, string, ...HandlerFunc) IRoutes
	Any(string, ...HandlerFunc) IRoutes
	GET(string, ...HandlerFunc) IRoutes
	POST(string, ...HandlerFunc) IRoutes
	DELETE(string, ...HandlerFunc) IRoutes
	PATCH(string, ...HandlerFunc) IRoutes
	PUT(string, ...HandlerFunc) IRoutes
	OPTIONS(string, ...HandlerFunc) IRoutes
	HEAD(string, ...HandlerFunc) IRoutes
	MATCH([]string, string, ...HandlerFunc) IRoutes
	StaticFile(string, string) IRoutes
	StaticFileFS(string, string, http.FileSystem) IRoutes
	Static(string, string) IRoutes
	StaticFS(string, http.FileSystem) IRoutes
}
type RouterGroup struct {
	Handlers HandlersChain
	basePath string
	engine   *Engine
	root     bool
}

var _ IRouter = (*RouterGroup)(nil)

func (group *RouterGroup) Use(middleware ...HandlerFunc) IRoutes {
	group.Handlers = append(group.Handlers, middleware...)
	return group.returnObj()
}
func (group *RouterGroup) Group(relativePath string, handlers ...HandlerFunc)*RouterGroup{
	return &RouterGroup{
		Handlers: group.combieHandlers(handlers),
		basePath: group.calculateAbsolutePath(relativePath),
		engine:group.engine
	}
}

func(group *RouterGroup)BasePath()string{
	return group.basePath
}
func(group *RouterGroup)handle(httpMethod,relativePath string,handlers HandlersChain)IRoutes{
	absolutePath:=group.calculateAbsolutePath(relativePath)
	handlers =group.combineHanders(handlers)
	group.engine.addRoute(httpMethod,absolutePath,handlers)
	return group.returnObj()
}
func(group *RouterGroup)Handler(httpMethod,relativePath string,handlers ...HandlerFunc)IRoutes{
	if matched:=regEnLetter.MatchString(httpMethod);!matched{
		panic("http method " + httpMethod + " is not valid")
	}
	return group.handle(httpMethod,relativePath,handlers)
}
func(group *RouterGroup)POST(relativePath string,handlers ...HandlerFunc)IRoutes{
	return group.handle(http.MethodPost,relativePath,handlers)
}
func(group *RouterGroup)GET(relativePath string,handlers ...HandlerFunc)IRoutes{
	return group.handle(http.MethodGet,relativePath,handlers)
}
func(group *RouterGroup)DELETE(relativePath string, handlers ...HandlerFunc)IRoutes{
	return group.handle(http.MethodDelete,relativePath,handlers)
}
func(group *RouterGroup)PATCH(relativePath string,handlers ...HandlerFunc)IRoutes{
	return group.handle(http.MethodPatch,relativePath,handlers)
}
func(group *RouterGroup)PUT(relativePath string,handlers ...HandlerFunc)IRoutes{
	return group.handle(http.MethodPut,relativePath,handlers)
}
func (group *RouterGroup) OPTIONS(relativePath string, handlers ...HandlerFunc) IRoutes {
	return group.handle(http.MethodOptions, relativePath, handlers)
}

func (group *RouterGroup) HEAD(relativePath string, handlers ...HandlerFunc) IRoutes {
	return group.handle(http.MethodHead, relativePath, handlers)
}
func(group *RouterGroup)Any(relativePath string,handlers ...HandlerFunc)IRoutes{
	for _,method:=range anyMethods{
		group.handle(method,relativePath,handlers)
	}
	return group.returnObj()
}
func(group *RouterGroup)Match(methods []string,relativePath string,handlers ...HandlerFunc)IRoutes{
	for _,method:=range methods{
		group.Handler(method,relativePath,handlers)
	}
	return group.returnObj()
}
func(group *RouterGroup)StaticFile(relativePath,filepath string)IRoutes{
	return group.staticFileHandler(relativePath, func(c *Context) {
		c.File(filepath)
	})
}
func(group *RouterGroup)StaticFileFS(relativePath,filepath string,fs http.FileSystem)IRoutes{
	return group.staticFileHandler(relativePath, func(c *Context) {
		c.FileFromFS(filepath,fs)
	})
}
func(group *RouterGroup)staticFileHandler(relativePath string,handler HandlerFunc)IRoutes{
	if strings.Contains(relativePath,":")||strings.Contains(relativePath,"*"){
		panic(any("URL parameters can not be used when serving a static file"))
	}
	group.GET(relativePath,handler)
	group.HEAD(relativePath,handler)
	return group.returnObj()
}
func(group *RouterGroup)Static(relativePath,root string)IRoutes{
	return group.StaticFileFS(relativePath,Dir(root,false))
}
func(group *RouterGroup)StaticFS(relativePath string,fs http.FileSystem)IRoutes{
    if strings.Contains(relativePath,":")||strings.Contains(relativePath,"*"){
		panic(any("URL parameters can not be used when serving a static folder"))
	}
	handler:=group.createStaticHandler(relativePath,fs)
	urlPattern:=path.Join(relativePath,"/*filepath")
    group.GET(urlPattern,handler)
	group.HEAD(urlPattern,handler)
	return group.returnObj()
}
func(group *RouterGroup)createStaticHandler(relativePath string,fs http.FileSystem)HandlerFunc{
	absolutePath:=group.calulateAbsolutePath(relativePath)
	fileServer:=http.StripPrefix(absolutePath,http.FileServer(fs))

	return func(c *Context) {
		if _,noListing:=fs.(*onlyFilesFS);noListing{
			c.Writer.WriteHeader(http.StatusNotFound)
		}
		file:=c.Param("filepath")
		f,err:=fs.Open(file)
		if err!=nil{
			c.Writer.WriteHeader(http.StatusNotFound)
			c.handlers=group.engine.noRoute
			c.index=-1
			return
		}
		f.Close()
		fileServer.ServeHTTP(c.Writer,c.Request)
	}

}
// how to
func(group *RouterGroup)combineHandlers(handlers HandlersChain)HandlersChain{
	finalSize:=len(group.Handlers)+len(handlers)
	assert1(finalSize<int(abortIndex),"too many handlers")
	mergedHandlers:=make(HandlersChain,finalSize)
	copy(mergedHandlers,group.Handlers)
	copy(mergedHandlers[len(group.Handlers):],handlers)
	return mergedHandlers
}

func(group *RouterGroup)calculateAbsolutePath(relativePath string)string{
	return joinPaths(group.basePath,relativePath)
}
func(group *RouterGroup)returnObj()IRoutes{
	 if group.root{
		 return group.engine
	 }
	 return group
}