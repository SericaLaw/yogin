package yogin

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"syscall"
	"testing"
)

func TestPanicInHandler(t *testing.T) {
	{
		r := Default()
		r.GET("/recovery", func(_ *Context) {
			panic("Oupps, Houston, we have a problem")
		})

		req := httptest.NewRequest(http.MethodGet, "/recovery", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	}

	{
		r := Default()
		r.GET("/recovery", func(c *Context) {
			c.AbortWithStatus(http.StatusBadRequest)
			panic("Oupps, Houston, we have a problem")
		})

		req := httptest.NewRequest(http.MethodGet, "/recovery", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	}

	{
		r := Default()
		r.GET("/recovery", func(c *Context) {
			c.Header("X-Test", "Value")
			c.Status(http.StatusNoContent)

			err := &net.OpError{Err: &os.SyscallError{Err: syscall.EPIPE}}
			panic(err)
		})

		req := httptest.NewRequest(http.MethodGet, "/recovery", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNoContent, w.Code)
	}

	{
		r := Default()
		r.GET("/recovery", func(c *Context) {
			c.Header("X-Test", "Value")
			c.Status(http.StatusNoContent)

			err := &net.OpError{Err: &os.SyscallError{Err: syscall.ECONNRESET}}
			panic(err)
		})

		req := httptest.NewRequest(http.MethodGet, "/recovery", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNoContent, w.Code)
	}
}

func TestLotsOfPanic(t *testing.T) {
	r := Default()
	r.GET("/recovery", func(_ *Context) {
		panic("Oupps, Houston, we have a problem")
	})
	r.GET("/normal", func(c *Context) {
		c.String(http.StatusOK, "normal")
	})

	for i := 0; i < 5; i++ {
		{
			req := httptest.NewRequest(http.MethodGet, "/recovery", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			assert.Equal(t, http.StatusInternalServerError, w.Code)
		}

		{
			req := httptest.NewRequest(http.MethodGet, "/normal", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, "normal", w.Body.String())
		}
	}
}

func TestLoginSuccess(t *testing.T) {
	accounts := Accounts{"admin": "password"}
	r := Default()
	r.Use(BasicAuth(accounts))
	r.GET("/login", func(c *Context) {
		c.String(http.StatusOK, c.MustGet(AuthUserKey).(string))
	})

	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	req.Header.Set("Authorization", secretHash("admin", "password"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "admin", w.Body.String())
}

func TestLoginFailure(t *testing.T) {
	called := false
	r := Default()
	r.Use(BasicAuth(Accounts{"admin": "password"}))
	r.GET("/login", func(c *Context) {
		called = true
		c.String(http.StatusOK, c.MustGet(AuthUserKey).(string))
	})

	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	req.Header.Set("Authorization", secretHash("admin", "passwd"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.False(t, called)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Equal(t, basicRealm, w.Header().Get("WWW-Authenticate"))
}

func TestBasicAuth(t *testing.T) {
	accounts := Accounts{
		"foo":    "bar",
		"austin": "1234",
		"lena":   "hello2",
		"manu":   "4321",
	}
	secrets := map[string]interface{}{
		"foo":    map[string]interface{}{"email": "foo@bar.com", "phone": "123433"},
		"austin": map[string]interface{}{"email": "austin@example.com", "phone": "666"},
		"lena":   map[string]interface{}{"email": "lena@guapa.com", "phone": "523443"},
	}
	const noSecret = "NO SECRET :("
	const publicInfo = "PUBLIC INFO :)"

	r := Default()

	// login interface, return secret token
	r.POST("/login", func(c *Context) {
		user := c.PostForm("user")
		password := c.PostForm("password")
		if pwd, ok := accounts[user]; ok {
			if password == pwd {
				c.String(http.StatusOK, secretHash(user, password))
				return
			}
		}
		c.Forbidden().WithString("wrong user or password")
	})

	// routes under authorized group requires user to login first
	authorized := r.Group("/admin", BasicAuth(accounts))

	authorized.GET("/secrets", func(c *Context) {
		// get user, it was set by the BasicAuth middleware
		user := c.MustGet(AuthUserKey).(string)
		if secret, ok := secrets[user]; ok {
			c.OK().WithJSON(H{"user": user, "secret": secret})
		} else {
			c.OK().WithJSON(H{"user": user, "secret": noSecret})
		}
	})

	// routes under public group requires no auth
	public := r.Group("/public")

	public.GET("/info", func(c *Context) {
		c.String(http.StatusOK, publicInfo)
	})

	// test legal users
	for user, password := range accounts {
		var token string
		{
			req := httptest.NewRequest(http.MethodPost, "/login", nil)
			req.PostForm = make(url.Values)
			req.PostForm["user"] = []string{user}
			req.PostForm["password"] = []string{password}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			token = w.Body.String()

			assert.Equal(t, http.StatusOK, w.Code)
		}

		{
			req := httptest.NewRequest(http.MethodGet, "/admin/secrets", nil)
			req.Header.Set("Authorization", token)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			var obj map[string]interface{}
			assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &obj))

			var expectedSecret interface{}
			if secret, ok := secrets[user]; ok {
				expectedSecret = secret
			} else {
				expectedSecret = noSecret
			}

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, user, obj["user"])
			assert.Equal(t, expectedSecret, obj["secret"])
		}

		{
			req := httptest.NewRequest(http.MethodGet, "/public/info", nil)
			req.Header.Set("Authorization", token)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, publicInfo, w.Body.String())
		}
	}

	// test illegal user
	var token string
	{
		req := httptest.NewRequest(http.MethodPost, "/login", nil)
		req.PostForm = make(url.Values)
		req.PostForm["user"] = []string{"unauthorized"}
		req.PostForm["password"] = []string{"unauthorized"}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		token = w.Body.String()

		assert.Equal(t, http.StatusForbidden, w.Code)
	}

	{
		req := httptest.NewRequest(http.MethodGet, "/admin/secrets", nil)
		req.Header.Set("Authorization", token)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Equal(t, basicRealm, w.Header().Get("WWW-Authenticate"))
	}

	{
		req := httptest.NewRequest(http.MethodGet, "/public/info", nil)
		req.Header.Set("Authorization", token)

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, publicInfo, w.Body.String())
	}
}
