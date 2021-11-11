package yogin

import (
	"crypto/subtle"
	"encoding/base64"
	"net/http"
)

// AuthUserKey is the cookie name for user credential in basic auth.
const AuthUserKey = "user"
const basicRealm = "Basic basicRealm=\"Authorization Required\""

// Accounts defines a key/value for user/pass list of authorized logins.
type Accounts map[string]string

type authPair struct {
	value string
	user  string
}

type authPairs []authPair

func (a authPairs) searchCredential(authValue string) (string, bool) {
	if authValue == "" {
		return "", false
	}
	for _, pair := range a {
		if subtle.ConstantTimeCompare([]byte(pair.value), []byte(authValue)) == 1 {
			return pair.user, true
		}
	}
	return "", false
}

// BasicAuth returns a Basic HTTP Authorization middleware.
// It takes as arguments a map[string]string where the key is the user name and the value is the password.
// "Authorization Required" will be used as Realm.
// (see http://tools.ietf.org/html/rfc2617#section-1.2)
func BasicAuth(accounts Accounts) HandlerFunc {
	pairs := processAccounts(accounts)
	return func(c *Context) {
		// find and validate auth token, and get username from pairs
		user, found := pairs.searchCredential(c.GetHeader("Authorization"))
		if !found {
			c.Header("WWW-Authenticate", basicRealm)
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		// set username
		c.Set(AuthUserKey, user)
		c.Next()
	}
}

func processAccounts(accounts Accounts) authPairs {
	length := len(accounts)
	assert1(length > 0, "Empty list of authorized credentials")

	pairs := make(authPairs, 0, length)
	for user, password := range accounts {
		assert1(user != "", "User can not be empty")

		value := secretHash(user, password)
		pairs = append(pairs, authPair{
			value: value,
			user:  user,
		})
	}
	return pairs
}

func secretHash(user, password string) string {
	base := user + ":" + password
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(base))
}
