package yogin

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)


func TestRoutesBasic(t *testing.T) {
	r := New()
	methods := []string{http.MethodGet, http.MethodPost}

	{
		for _, method := range methods {
			r.addRoute(method, "/", HandlersChain{
				func(c *Context) { assert.Equal(t, "/", c.FullPath) },
				func(c *Context) { c.String(http.StatusOK, "%s %s matches %s", c.Method, c.Path, c.FullPath) },
			})
			r.addRoute(method, "/:hello", HandlersChain{
				func(c *Context) { assert.Equal(t, "/:hello", c.FullPath) },
				func(c *Context) { assert.Equal(t, "hello", c.Param("hello")) },
				func(c *Context) { c.OK().WithString("%s %s matches %s", c.Method, c.Path, c.FullPath) },
			})
			r.addRoute(method, "/:hello/:world", HandlersChain{
				func(c *Context) { assert.Equal(t, "/:hello/:world", c.FullPath) },
				func(c *Context) {
					if c.Method == http.MethodPost {
						assert.Equal(t, "Value", c.PostForm("key"))
					}
				},
				func(c *Context) { c.String(http.StatusOK, "%s %s matches %s", c.Method, c.Path, c.FullPath) },
			})
			r.addRoute(method, "/:hello/:world/*extra", HandlersChain{
				func(c *Context) { assert.Equal(t, "/:hello/:world/*extra", c.FullPath) },
				func(c *Context) {
					if c.Method == http.MethodPost {
						assert.Equal(t, "Value", c.PostForm("key"))
					}
				},
				func(c *Context) { c.OK().WithString("%s %s matches %s", c.Method, c.Path, c.FullPath) },
			})
		}
	}

	ts := httptest.NewServer(r)
	defer ts.Close()

	{
		res, err := http.Get(fmt.Sprintf("%s/", ts.URL))
		if err != nil {
			log.Println(err)
		}

		resp, _ := ioutil.ReadAll(res.Body)
		assert.Equal(t, http.StatusOK, res.StatusCode)
		fmt.Println(string(resp))
	}

	{
		res, err := http.Get(fmt.Sprintf("%s/hello", ts.URL))
		if err != nil {
			log.Println(err)
		}

		resp, _ := ioutil.ReadAll(res.Body)
		assert.Equal(t, http.StatusOK, res.StatusCode)
		fmt.Println(string(resp))
	}

	{
		res, err := http.PostForm(fmt.Sprintf("%s/hello/world", ts.URL), url.Values{"key": {"Value"}})
		if err != nil {
			log.Println(err)
		}

		resp, _ := ioutil.ReadAll(res.Body)
		assert.Equal(t, http.StatusOK, res.StatusCode)
		fmt.Println(string(resp))
	}

	{
		res, err := http.PostForm(fmt.Sprintf("%s/hello/world/from/client", ts.URL), url.Values{"key": {"Value"}})
		if err != nil {
			log.Println(err)
		}

		resp, _ := ioutil.ReadAll(res.Body)
		assert.Equal(t, http.StatusOK, res.StatusCode)
		fmt.Println(string(resp))
	}
}

func TestRoutesConflict(t *testing.T) {
	t0 := func() {
		r := New()
		{
			r.addRoute(http.MethodGet, "/hello", nil)
			r.addRoute(http.MethodGet, "/hello", nil)
		}
	}
	assert.Panics(t, t0)

	t1 := func() {
		r := New()
		{
			r.addRoute(http.MethodGet, "/:hello/:world", nil)
			r.addRoute(http.MethodGet, "/:hello/*world", nil)
		}
	}
	assert.Panics(t, t1)

	t2 := func() {
		r := New()
		{
			r.addRoute(http.MethodGet, "/:hello/*world", nil)
			r.addRoute(http.MethodGet, "/:hello/:world", nil)

		}
	}
	assert.Panics(t, t2)

	t3 := func() {
		r := New()
		{
			r.addRoute(http.MethodGet, "/:hello/:world1", nil)
			r.addRoute(http.MethodGet, "/:hello/:world2", nil)
		}
	}
	assert.Panics(t, t3)

	t4 := func() {
		r := New()
		{
			r.addRoute(http.MethodGet, "/*hello/:world", nil)
		}
	}
	assert.Panics(t, t4)

	t5 := func() {
		r := New()
		{
			r.addRoute(http.MethodGet, "/*hello/*world", nil)
		}
	}
	assert.Panics(t, t5)

	t6 := func() {
		r := New()
		{
			r.addRoute(http.MethodGet, "/*hello", nil)
			r.addRoute(http.MethodGet, "/:hello/*world", nil)
		}
	}
	assert.Panics(t, t6)

	t7 := func() {
		r := New()
		{
			r.addRoute(http.MethodGet, "/hello/*world", nil)
			r.addRoute(http.MethodGet, "/*hello", nil)
		}
	}
	assert.Panics(t, t7)

	t8 := func() {
		r := New()
		{
			r.addRoute(http.MethodGet, "/*hello", nil)
			r.addRoute(http.MethodGet, "/:hello", nil)
		}
	}
	assert.Panics(t, t8)

	// allow at most one :param style segment at each position
	t9 := func() {
		r := New()
		{
			r.addRoute(http.MethodGet, "/:hello1/world1", nil)
			r.addRoute(http.MethodGet, "/:hello2/world2", nil)
		}
	}
	assert.Panics(t, t9)
}

func TestRoutesNoConflict(t *testing.T) {
	t0 := func() {
		r := New()
		{
			r.addRoute(http.MethodGet, "/hello", nil)
			r.addRoute(http.MethodGet, "/:hello", nil)
		}
	}
	assert.NotPanics(t, t0)

	t1 := func() {
		r := New()
		{
			r.addRoute(http.MethodGet, "/*hello", nil)
			r.addRoute(http.MethodPost, "/hello/*world", nil)
		}
	}
	assert.NotPanics(t, t1)

	t2 := func() {
		r := New()
		{
			r.addRoute(http.MethodGet, "/hello/:world", nil)
			r.addRoute(http.MethodGet, "/:hello/world", nil)
			r.addRoute(http.MethodGet, "/:hello/:world", nil)
		}
	}
	assert.NotPanics(t, t2)
}

func TestRoutesMore(t *testing.T) {
	r := New()
	{
		r.addRoute(http.MethodGet, "/:hello/:world", HandlersChain{
			func(c *Context) { c.String(http.StatusOK, c.FullPath) },
		})
		r.addRoute(http.MethodGet, "/hello/:world", HandlersChain{
			func(c *Context) { c.String(http.StatusOK, c.FullPath) },
		})
		r.addRoute(http.MethodGet, "/:hello/world", HandlersChain{
			func(c *Context) { c.String(http.StatusOK, c.FullPath) },
		})
	}

	{
		req := httptest.NewRequest(http.MethodGet, "/hello/world", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, "/hello/:world", w.Body.String())
	}

	{
		req := httptest.NewRequest(http.MethodGet, "/hello1/world", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, "/:hello/world", w.Body.String())
	}

	{
		req := httptest.NewRequest(http.MethodGet, "/hello1/world1", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, "/:hello/:world", w.Body.String())
	}
}
