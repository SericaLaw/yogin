package yogin

import (
	"encoding/json"
	"fmt"
	"math"
	"net"
	"net/http"
	"strings"
	"sync"
)

type Context struct {
	Writer 	http.ResponseWriter
	Request	*http.Request

	// middlewares
	handlers	HandlersChain
	index 		int8	// current middleware

	// request info
	Path 	 	string
	Method   	string
	ClientIP 	string

	Params 		Params
	FullPath 	string

	// response info
	statusCode 	int
	bodySize	int

	// Errors is a list of errors attached to all the handlers/middlewares who used this context.
	Errors 	[]error

	// This mutex protect Keys map
	mu	 	sync.RWMutex

	// Keys is a key/value pair exclusively for the context of each request.
	Keys 	map[string]interface{}
	engine 	*Engine
}

func (c *Context) reset(w http.ResponseWriter, req *http.Request) *Context {
	ip, _, err := net.SplitHostPort(strings.TrimSpace(req.RemoteAddr))
	if err != nil {
		fmt.Printf("remote ip parse error: %v\n", err)
	}

	c.Writer = w
	c.Request = req

	c.Path = req.URL.Path
	c.Method = req.Method
	c.ClientIP = ip

	c.handlers = nil
	c.index = -1

	c.Params = c.Params[:0]
	c.FullPath = ""

	c.statusCode = 0
	c.bodySize = 0

	c.Errors = c.Errors[:0]
	c.Keys = nil
	return c
}

/************************************/
/************ INPUT DATA ************/
/************************************/

// Param returns the value of the URL param.
// It is a shortcut for c.Params.ByName(key)
//     router.GET("/user/:id", func(c *gin.Context) {
//         // a GET request to /user/john
//         id := c.Param("id") // id == "john"
//     })
func (c *Context) Param(key string) string {
	return c.Params.ByName(key)
}

// Query returns the keyed url query value if it exists,
// otherwise it returns an empty string `("")`.
// It is shortcut for `c.Request.URL.Query().Get(key)`
//     GET /path?id=1234&name=Manu&value=
// 	   c.Query("id") == "1234"
// 	   c.Query("name") == "Manu"
// 	   c.Query("value") == ""
// 	   c.Query("wtf") == ""
func (c *Context) Query(key string) string {
	return c.Request.URL.Query().Get(key)
}

// PostForm returns the specified key from a POST urlencoded form or multipart form
// when it exists, otherwise it returns an empty string `("")`.
func (c *Context) PostForm(key string) string {
	return c.Request.FormValue(key)
}

// GetHeader returns value from request headers.
func (c *Context) GetHeader(key string) string {
	return c.Request.Header.Get(key)
}

/************************************/
/************* RESPONSE *************/
/************************************/

// Header is a intelligent shortcut for c.Writer.Header().Set(key, value).
// It writes a header in the response.
// If value == "", this method removes the header `c.Writer.Header().Del(key)`
func (c *Context) Header(key, value string) {
	if value == "" {
		c.Writer.Header().Del(key)
		return
	}
	c.Writer.Header().Set(key, value)
}

// Status sets the HTTP response code.
// It should be set only once before sending out response.
func (c *Context) Status(code int) {
	if code > 0 && c.statusCode != 0 && c.statusCode != code {
		c.Error(fmt.Errorf("%s[WARNING]%s Headers were already written. Wanted to override status code %d with %d, rejected", yellow, reset, c.statusCode, code))
		return
	}
	c.statusCode = code
	c.Writer.WriteHeader(code)
}

func (c *Context) OK() *Context {
	c.Status(http.StatusOK)
	return c
}

func (c *Context) NotFound() *Context {
	c.Status(http.StatusNotFound)
	return c
}

func (c *Context) Forbidden() *Context {
	c.Status(http.StatusForbidden)
	return c
}

var (
	jsonContentType		= []string{"application/json; charset=utf-8"}
	htmlContentType 	= []string{"text/html; charset=utf-8"}
	plainContentType 	= []string{"text/plain; charset=utf-8"}
)

func (c *Context) WithString(format string, values ...interface{}) *Context {
	writeContentType(c.Writer, plainContentType)
	data := []byte(fmt.Sprintf(format, values...))
	wc, _ := c.Writer.Write(data)
	c.bodySize += wc
	return c
}

// String writes the given string into the response body.
func (c *Context) String(code int, format string, values ...interface{}) {
	c.Status(code)
	c.WithString(format, values...)
}

func (c *Context) WithJSON(obj interface{}) *Context {
	writeContentType(c.Writer, jsonContentType)
	data, err := json.Marshal(obj)
	if err != nil {
		panic(err)
	}
	wc, _ := c.Writer.Write(data)
	c.bodySize += wc
	return c
}

// JSON serializes the given struct as JSON into the response body.
// It also sets the Content-Type as "application/json".
func (c *Context) JSON(code int, obj interface{}) {
	c.Status(code)
	c.WithJSON(obj)
}

func (c *Context) WithHTML(name string, obj interface{}) *Context {
	writeContentType(c.Writer, htmlContentType)
	var b strings.Builder
	if err := c.engine.htmlTemplates.ExecuteTemplate(&b, name, obj); err != nil {
		panic(err)
	}
	data := []byte(b.String())
	wc, _ := c.Writer.Write(data)
	c.bodySize += wc
	return c
}

// HTML renders the HTTP template specified by its file name.
// It also updates the HTTP code and sets the Content-Type as "text/html".
// See http://golang.org/doc/articles/wiki/
func (c *Context) HTML(code int, name string, obj interface{}) {
	c.Status(code)
	c.WithHTML(name, obj)
}

// File writes the specified file into the body stream in an efficient way.
func (c *Context) File(filepath string) {
	http.ServeFile(c.Writer, c.Request, filepath)
}

func writeContentType(w http.ResponseWriter, value []string) {
	header := w.Header()
	if val := header["Content-Type"]; len(val) == 0 {
		header["Content-Type"] = value
	}
}

/************************************/
/********* ERROR MANAGEMENT *********/
/************************************/

// Error attaches an error to the current context. The error is pushed to a list of errors.
// It's a good idea to call Error for each error that occurred during the resolution of a request.
// A middleware can be used to collect all the errors and push them to a database together,
// print a log, or append it in the HTTP response.
// Error will panic if err is nil.
func (c *Context) Error(err error) {
	assert1(err != nil, "err is nil")
	c.Errors = append(c.Errors, err)
}

/************************************/
/*********** FLOW CONTROL ***********/
/************************************/

const abortIndex int8 = math.MaxInt8 >> 1

// Next should be used only inside middleware.
// It executes the pending handlers in the chain inside the calling handler.

func (c *Context) Next() {
	c.index++
	s := int8(len(c.handlers))
	for c.index < s {
		c.handlers[c.index](c)
		c.index++
	}
}

// Abort prevents pending handlers from being called. Note that this will not stop the current handler.
// Let's say you have an authorization middleware that validates that the current request is authorized.
// If the authorization fails (ex: the password does not match), call Abort to ensure the remaining handlers
// for this request are not called.
func (c *Context) Abort() {
	c.index = abortIndex
}

// AbortWithStatus calls `Abort()` and writes the headers with the specified status code.
// For example, a failed attempt to authenticate a request could use: context.AbortWithStatus(401).
func (c *Context) AbortWithStatus(code int) {
	c.Status(code)
	c.Abort()
}

func (c *Context) IsAborted() bool {
	return c.index >= abortIndex
}

/************************************/
/******** METADATA MANAGEMENT********/
/************************************/

// Set is used to store a new key/value pair exclusively for this context.
// It also lazy initializes  c.Keys if it was not used previously.
func (c *Context) Set(key string, value interface{}) {
	c.mu.Lock()
	if c.Keys == nil {
		c.Keys = make(map[string]interface{})
	}

	c.Keys[key] = value
	c.mu.Unlock()
}

// Get returns the value for the given key, ie: (value, true).
// If the value does not exists it returns (nil, false)
func (c *Context) Get(key string) (value interface{}, exists bool) {
	c.mu.RLock()
	value, exists = c.Keys[key]
	c.mu.RUnlock()
	return
}

// MustGet returns the value for the given key if it exists, otherwise it panics.
func (c *Context) MustGet(key string) interface{} {
	if value, exists := c.Get(key); exists {
		return value
	}
	panic(fmt.Sprintf("Key %s does not exist in context", key))
}
