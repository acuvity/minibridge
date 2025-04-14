package backend

import (
	"go.acuvity.ai/minibridge/pkgs/policer"
)

type wsCfg struct {
	policer    policer.Policer
	dumpStderr bool
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

// OptWSDumpStderrOnError controls whether the WS server should
// dump the stderr of the MCP server as is, or in a log.
func OptWSDumpStderrOnError(dump bool) OptWS {
	return func(cfg *wsCfg) {
		cfg.dumpStderr = dump
	}
}
