package backend

import (
	"go.acuvity.ai/minibridge/pkgs/policer"
)

type wsCfg struct {
	policer policer.Policer
}

func newWSCfg() wsCfg {
	return wsCfg{}
}

// OptWS are options that can be given to NewStdio().
type OptWS func(*wsCfg)

// OptWSPolicer sets the Policer to forward the traffic to.
func OptWSPolicer(policer policer.Policer) OptWS {
	return func(cfg *wsCfg) {
		cfg.policer = policer
	}
}
