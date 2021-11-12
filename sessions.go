package yogin

import (
	"github.com/gorilla/sessions"
)

var (
	key = []byte("super-secret-key")
	store = sessions.NewCookieStore(key)
)

const DefaultSessionKey = "session"

func Session(name string) HandlerFunc {
	return func(c *Context) {
		session, _ := store.Get(c.Request, name)
		c.Set(DefaultSessionKey, session)
		c.Next()
	}
}

func Sessions(names []string) HandlerFunc {
	return func(c *Context) {
		sessions := make(map[string]*sessions.Session)
		for _, name := range names {
			session, _ := store.Get(c.Request, name)
			sessions[name] = session
		}
		c.Set(DefaultSessionKey, sessions)
		c.Next()
	}
}
