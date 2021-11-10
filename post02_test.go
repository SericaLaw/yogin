package yogin

import (
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRouterGroupBasic(t *testing.T) {
	r := New()
	g0 := r.Group("/hola", func(c *Context) {})
	g0.Use(func(c *Context) {})

	assert.Len(t, g0.Handlers, 2)
	assert.Equal(t, "/hola", g0.BasePath())
	assert.Equal(t, r, g0.engine)

	g1 := g0.Group("manu")
	g1.Use(Logger(), func (c *Context) {}, func(c *Context) {})

	assert.Len(t, g1.Handlers, 5)
	assert.Equal(t, "/hola/manu", g1.BasePath())
	assert.Equal(t, r, g1.engine)
}

func TestRouterGroupHandle(t *testing.T) {
	r := New()
	r.Use(Logger())
	v1 := r.Group("v1", func(c *Context) {})
	assert.Equal(t, "/v1", v1.BasePath())

	login := v1.Group("/login", func(c *Context) {}, func(c *Context) {})
	assert.Equal(t, "/v1/login", login.BasePath())

	handler := func(c *Context) {
		c.String(http.StatusBadRequest, "the method was %s and index %d", c.Request.Method, c.index)
	}

	r.GET("/test", handler)
	r.POST("/test", handler)
	v1.GET("/test", handler)
	v1.POST("/test", handler)
	login.POST("/test", handler)
	login.GET("/test", handler)

	methods := []string{http.MethodGet, http.MethodPost}
	for _, method := range methods {
		{
			req := httptest.NewRequest(method, "/test", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			assert.Equal(t, http.StatusBadRequest, w.Code)
			assert.Equal(t, "the method was "+method+" and index 1", w.Body.String())
		}

		{
			req := httptest.NewRequest(method, "/v1/test", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			assert.Equal(t, http.StatusBadRequest, w.Code)
			assert.Equal(t, "the method was "+method+" and index 2", w.Body.String())
		}

		{
			req := httptest.NewRequest(method, "/v1/login/test", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			assert.Equal(t, http.StatusBadRequest, w.Code)
			assert.Equal(t, "the method was "+method+" and index 4", w.Body.String())
		}

		{
			req := httptest.NewRequest(method, "/v1/admin/test", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			assert.Equal(t, http.StatusNotFound, w.Code)
		}
	}
}
