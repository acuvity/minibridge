package backend

import (
	"go.acuvity.ai/bahamut"
	"go.acuvity.ai/minibridge/pkgs/policer"
	"go.acuvity.ai/minibridge/pkgs/scan"
)

type wsCfg struct {
	policer    policer.Policer
	dumpStderr bool
	corsPolicy *bahamut.CORSPolicy
	sbom       scan.SBOM
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

// OptWSCORSPolicy sets the bahamut.CORSPolicy to use for
// connection originating from a webrowser.
func OptWSCORSPolicy(policy *bahamut.CORSPolicy) OptWS {
	return func(cfg *wsCfg) {
		cfg.corsPolicy = policy
	}
}

// OptSBOM sets a the utils.SBOM to use to verify
// server integrity.
func OptSBOM(sbom scan.SBOM) OptWS {
	return func(cfg *wsCfg) {
		cfg.sbom = sbom
	}
}
