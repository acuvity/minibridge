package backend

import (
	"go.acuvity.ai/a3s/pkgs/token"
	"go.acuvity.ai/bahamut"
	"go.acuvity.ai/minibridge/pkgs/policer"
)

type wsCfg struct {
	policer            policer.Policer
	dumpStderr         bool
	jwtJWKS            *token.JWKS
	jwtRequiredIss     string
	jwtRequiredAud     string
	jwtPrincipalClaims string
	corsPolicy         *bahamut.CORSPolicy
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

// OptWSAuth configures the needed information to enable authentication.
func OptWSAuth(jwks *token.JWKS, principalClaim string, requiredIssuer string, requiredAudience string) OptWS {
	return func(cfg *wsCfg) {
		cfg.jwtJWKS = jwks
		cfg.jwtRequiredIss = requiredIssuer
		cfg.jwtRequiredAud = requiredAudience
		cfg.jwtPrincipalClaims = principalClaim
	}
}

// OptWSCORSPolicy sets the bahamut.CORSPolicy to use for
// connection originating from a webrowser.
func OptWSCORSPolicy(policy *bahamut.CORSPolicy) OptWS {
	return func(cfg *wsCfg) {
		cfg.corsPolicy = policy
	}
}
