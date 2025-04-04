package backend

import (
	"crypto/tls"
)

type wsCfg struct {
	policerURL       string
	policerToken     string
	policerTLSConfig *tls.Config
}

func newWSCfg() wsCfg {
	return wsCfg{}
}

// OptWS are options that can be given to NewStdio().
type OptWS func(*wsCfg)

// OptWSPolicerURL sets the Policer URL to forward the traffic to.
func OptWSPolicerURL(url string, token string) OptWS {
	return func(cfg *wsCfg) {
		cfg.policerURL = url
		cfg.policerToken = token
	}
}

// OptWSPolicerTLSConfig sets the *tls.Config to use to
// contact the Policer.
func OptWSPolicerTLSConfig(tlsConfig *tls.Config) OptWS {
	return func(cfg *wsCfg) {
		cfg.policerTLSConfig = tlsConfig
	}
}
