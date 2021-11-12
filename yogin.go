package yogin

import (
	"html/template"
	"net/http"
	"sync"
)

type HandlersChain []HandlerFunc
type HandlerFunc func(*Context)
type H map[string]interface{}

var notFoundHandler = func(c *Context) {
	c.String(http.StatusNotFound, "url %v not found", c.Path)
}

type Engine struct {
	RouterGroup
	methodTrees map[string]methodTree
	contextPool	sync.Pool

	htmlTemplates *template.Template // for html render
	FuncMap       template.FuncMap   // for html render
}

func (engine *Engine) addRoute(method, path string, handlers HandlersChain) {
	if _, ok := engine.methodTrees[method]; !ok {
		engine.methodTrees[method] = methodTree{method, &node{path: "/"}}
	}
	tree := engine.methodTrees[method]
	tree.addRoute(path, handlers)
}

func (engine *Engine) allocateContext() *Context {
	v := make(Params, 0, 4)
	return &Context{Params: v, engine: engine}
}

// ServeHTTP conforms to the http.Handler interface.
func (engine *Engine) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	method := req.Method
	path := req.URL.Path
	c := engine.contextPool.Get().(*Context)
	c.reset(w, req)

	if _, ok := engine.methodTrees[method]; !ok {
		c.handlers = engine.RouterGroup.combineHandlers(HandlersChain{notFoundHandler})
		c.Next()
		return
	}
	tree := engine.methodTrees[method]

	value := tree.getRoute(path)
	if value.handlers == nil {
		c.handlers = engine.RouterGroup.combineHandlers(HandlersChain{notFoundHandler})
		c.Next()
		return
	}

	c.handlers = value.handlers
	c.Params = value.params
	c.FullPath = value.fullPath
	c.Next()

	engine.contextPool.Put(c)
}

// Run attaches the router to a http.Server and starts listening and serving HTTP requests.
// It is a shortcut for http.ListenAndServe(addr, router)
// Note: this method will block the calling goroutine indefinitely unless an error happens.
func (engine *Engine) Run(addr string) (err error) {
	err = http.ListenAndServe(addr, engine)
	return
}

func New() *Engine {
	engine := &Engine{
		RouterGroup: RouterGroup{
			Handlers: nil,
			basePath: "/",
			root:     true,
		},
		methodTrees: make(map[string]methodTree),
	}
	engine.RouterGroup.engine = engine
	engine.contextPool.New = func() interface{} {
		return engine.allocateContext()
	}
	return engine
}

func Default() *Engine {
	engine := New()
	engine.Use(Logger(), Recovery())
	return engine
}

func (engine *Engine) LoadHTMLGlob(pattern string) {
	engine.htmlTemplates = template.Must(template.New("").Funcs(engine.FuncMap).ParseGlob(pattern))
}

// SetFuncMap sets the FuncMap used for template.FuncMap.
func (engine *Engine) SetFuncMap(funcMap template.FuncMap) {
	engine.FuncMap = funcMap
}
