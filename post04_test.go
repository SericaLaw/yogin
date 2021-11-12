package yogin

import (
	"github.com/gorilla/sessions"
	"github.com/stretchr/testify/assert"
	"html/template"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"
)

func FormatAsDate(t time.Time) string {
	return t.Format("2006-01-02")
}

type person struct {
	Name string
	Age  int8
}

var people = [2]person{
	{Name: "Jack", Age: 20},
	{Name: "Rose", Age: 17},
}

func TestHTMLLoad(t *testing.T) {
	r := Default()
	r.SetFuncMap(template.FuncMap{
		"FormatAsDate": FormatAsDate,
	})
	r.LoadHTMLGlob("testdata/templates/*")
	r.Static("/", "./testdata/assets")
	r.StaticFile("/jack_and_rose.jpg", "./testdata/assets/img/jack_and_rose.jpg")

	r.GET("/", func(c *Context) {
		c.HTML(http.StatusOK, "hello.tmpl", H{
			"name": "yogin",
			"now": time.Now(),
			"people": people,
		})
	})

	//r.Run(":8080")
}

// try with: wrk -t16 -c500 -d30s http://localhost:8080/
func TestLimit(t *testing.T) {
	r := Default()
	r.Use(MaxAllowed(4))
	r.SetFuncMap(template.FuncMap{
		"FormatAsDate": FormatAsDate,
	})
	r.LoadHTMLGlob("testdata/templates/*")
	r.Static("/", "./testdata/assets")
	r.StaticFile("/jack_and_rose.jpg", "./testdata/assets/img/jack_and_rose.jpg")

	r.GET("/", func(c *Context) {
		c.HTML(http.StatusOK, "hello.tmpl", H{
			"name": "yogin",
			"now": time.Now(),
			"people": people,
		})
	})

	//r.Run(":8080")
}

func TestSession(t *testing.T)  {
	r := Default()
	r.Use(Session("auth"))

	const secret = "The cake is a lie!"
	{
		r.GET("/login", func(c *Context) {
			session := c.MustGet(DefaultSessionKey).(*sessions.Session)
			session.Values["authenticated"] = true
			session.Save(c.Request, c.Writer)
			c.String(http.StatusOK, "login success")
		})

		r.GET("/logout", func(c *Context) {
			session := c.MustGet(DefaultSessionKey).(*sessions.Session)
			session.Values["authenticated"] = false
			session.Save(c.Request, c.Writer)
			c.String(http.StatusOK, "logout success")
		})

		r.GET("/secret", func(c *Context) {
			session := c.MustGet(DefaultSessionKey).(*sessions.Session)

			if auth, ok := session.Values["authenticated"].(bool); !ok || !auth {
				c.String(http.StatusForbidden, "login first")
				return
			}

			c.String(http.StatusOK, secret)
		})
	}
	var cookie *http.Cookie
	{
		req := httptest.NewRequest(http.MethodGet, "/login", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "login success", w.Body.String())
		assert.Len(t, w.Result().Cookies(), 1)
		cookie = w.Result().Cookies()[0]
	}

	{
		req := httptest.NewRequest(http.MethodGet, "/secret", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, "login first", w.Body.String())
	}

	{
		req := httptest.NewRequest(http.MethodGet, "/secret", nil)
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, secret, w.Body.String())
	}

	{
		req := httptest.NewRequest(http.MethodGet, "/logout", nil)
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "logout success", w.Body.String())
		assert.Len(t, w.Result().Cookies(), 1)
		cookie = w.Result().Cookies()[0]
	}

	{
		req := httptest.NewRequest(http.MethodGet, "/secret", nil)
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, "login first", w.Body.String())
	}
}

func TestMoreSessions(t *testing.T)  {
	r := Default()
	r.Use(Sessions([]string{"auth", "incr"}))

	{
		r.GET("/login", func(c *Context) {
			sessions := c.MustGet(DefaultSessionKey).(map[string]*sessions.Session)
			auth := sessions["auth"]
			incr := sessions["incr"]
			auth.Values["authenticated"] = true
			auth.Save(c.Request, c.Writer)
			incr.Values["counter"] = 0
			incr.Save(c.Request, c.Writer)
			c.String(http.StatusOK, "login success")
		})

		r.GET("/incr", func(c *Context) {
			sessions := c.MustGet(DefaultSessionKey).(map[string]*sessions.Session)
			auth := sessions["auth"]
			incr := sessions["incr"]
			assert.True(t, auth.Values["authenticated"].(bool))
			cnt, ok := incr.Values["counter"].(int)
			assert.True(t, ok)
			incr.Values["counter"] = cnt + 1

			incr.Save(c.Request, c.Writer)
			c.String(http.StatusOK, strconv.Itoa(incr.Values["counter"].(int)))
		})
	}
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(wg *sync.WaitGroup) {
			defer wg.Done()
			var cookies []*http.Cookie
			var authCookie, incrCookie *http.Cookie
			{
				req := httptest.NewRequest(http.MethodGet, "/login", nil)
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)

				assert.Equal(t, http.StatusOK, w.Code)
				assert.Len(t, w.Result().Cookies(), 2)
				cookies = w.Result().Cookies()
				for _, cookie := range cookies {
					if cookie.Name == "auth" {
						authCookie = cookie
					} else {
						incrCookie = cookie
					}
				}
			}

			for i := 1; i < 10; i++ {
				req := httptest.NewRequest(http.MethodGet, "/incr", nil)
				req.AddCookie(authCookie)
				req.AddCookie(incrCookie)

				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)

				assert.Equal(t, http.StatusOK, w.Code)
				assert.Len(t, w.Result().Cookies(), 1)
				cookies = w.Result().Cookies()
				assert.Equal(t, strconv.Itoa(i), w.Body.String())

				for _, cookie := range cookies {
					if cookie.Name == "auth" {
						authCookie = cookie
					} else {
						incrCookie = cookie
					}
				}
			}
		}(&wg)
	}

	wg.Wait()
}
