package auth

import (
	"encoding/base64"
	"fmt"
)

// AuthScheme represents the various auth schemes.
type AuthScheme int

// Supported version of auth schemes.
const (
	AuthSchemeBasic AuthScheme = iota
	AuthSchemeBearer
)

// Auth holds user credentials.
type Auth struct {
	mode     AuthScheme
	user     string
	password string
}

// NewBasicAuth returns a new Basic Auth.
func NewBasicAuth(user string, password string) *Auth {
	return &Auth{
		mode:     AuthSchemeBasic,
		user:     user,
		password: password,
	}
}

// NewBearerAuth returns a new Bearer auth.
// User() will be set to "Bearer" and Password() ]
// will hold the token.
func NewBearerAuth(token string) *Auth {
	return &Auth{
		mode:     AuthSchemeBearer,
		user:     "Bearer",
		password: token,
	}
}

// Type returns the current type of Auth as a string.
func (a *Auth) Type() string {
	switch a.mode {
	case AuthSchemeBasic:
		return "Basic"
	case AuthSchemeBearer:
		return "Bearer"
	default:
		panic("unknown auth mode")
	}
}

// Encode encode the Auth to transmit on the wire.
func (a *Auth) Encode() string {
	switch a.mode {
	case AuthSchemeBasic:
		return fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString(fmt.Appendf([]byte{}, "%s:%s", a.user, a.password)))
	case AuthSchemeBearer:
		return fmt.Sprintf("Bearer %s", a.password)
	default:
		panic("unknown auth mode")
	}
}

// User returns the user.
func (a *Auth) User() string {
	return a.user
}

// Password returns the password.
func (a *Auth) Password() string {
	return a.password
}
