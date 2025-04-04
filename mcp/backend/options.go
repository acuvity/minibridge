package backend

import (
	"crypto/tls"
)

type wsCfg struct {
	apexURL       string
	apexToken     string
	apexTLSConfig *tls.Config
}

func newWSCfg() wsCfg {
	return wsCfg{}
}

// OptWS are options that can be given to NewStdio().
type OptWS func(*wsCfg)

// OptWSApexURL sets the apex URL to forward the traffic to.
func OptWSApexURL(url string, token string) OptWS {
	return func(cfg *wsCfg) {
		cfg.apexURL = url
		cfg.apexToken = token
	}
}

// OptWSApexURL sets the apex URL to forward the traffic to.
func OptWSApexTLSConfig(tlsConfig *tls.Config) OptWS {
	return func(cfg *wsCfg) {
		cfg.apexTLSConfig = tlsConfig
	}
}
