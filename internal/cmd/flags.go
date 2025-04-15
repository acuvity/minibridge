package cmd

import (
	"github.com/spf13/pflag"
)

var (
	fTLSClient   = pflag.NewFlagSet("tlsclient", pflag.ExitOnError)
	fTLSServer   = pflag.NewFlagSet("tlsserver", pflag.ExitOnError)
	fProfiler    = pflag.NewFlagSet("profile", pflag.ExitOnError)
	fHealth      = pflag.NewFlagSet("health", pflag.ExitOnError)
	fPolicer     = pflag.NewFlagSet("police", pflag.ExitOnError)
	fJWTVerifier = pflag.NewFlagSet("jwtverifier", pflag.ExitOnError)
	fCORS        = pflag.NewFlagSet("cors", pflag.ExitOnError)
	fAgentAuth   = pflag.NewFlagSet("agentauth", pflag.ExitOnError)

	initialized = false
)

func initSharedFlagSet() {

	if initialized {
		return
	}

	initialized = true

	fTLSServer.StringP("tls-server-cert", "c", "", "path to the server certificate for incoming HTTPS connections.")
	fTLSServer.StringP("tls-server-key", "k", "", "path to the key for the server certificate.")
	fTLSServer.StringP("tls-server-key-pass", "p", "", "passphrase for the server certificate key.")
	fTLSServer.String("tls-server-client-ca", "", "path to a CA to validate incoming client certificate. When enabled clients must send a valid certificate.")

	fTLSClient.StringP("tls-client-cert", "C", "", "path to the client certificate to authenticate against the minibridge backend.")
	fTLSClient.StringP("tls-client-key", "K", "", "path to the key for the client certificate.")
	fTLSClient.StringP("tls-client-key-pass", "P", "", "passphrase for the client certificate key.")
	fTLSClient.String("tls-client-backend-ca", "", "path to a CA to validate the minibridge backend server certificates.")
	fTLSClient.Bool("tls-client-insecure-skip-verify", false, "skip backend's server certificates validation. Do not do this.")

	fHealth.String("health-listen", ":8080", "listen address of the health server.")
	fHealth.Bool("health-enable", false, "enables health server.")

	fProfiler.String("profiling-listen", ":6060", "listen address of the health server.")
	fProfiler.Bool("profiling-enable", false, "enables profiling server.")

	fPolicer.StringP("policer-url", "U", "", "URL of the policer to POST agent policing requests.")
	fPolicer.StringP("policer-token", "T", "", "token to use to authenticate against the policer.")
	fPolicer.String("policer-ca", "", "path to a CA to validate the policer server certificates.")
	fPolicer.Bool("policer-insecure-skip-verify", false, "skip policer's server certificates validation. Do not do this.")

	fJWTVerifier.StringP("auth-jwks-url", "J", "", "enables authentication and requires agent to send JWTs signed by the given JWKS.")
	fJWTVerifier.String("auth-jwks-ca", "", "path to a CA to validate the JWKS server certificates.")
	fJWTVerifier.String("auth-jwt-cert", "", "enables authentication and requires agent to send JWTs signed by the given certificates.")
	fJWTVerifier.StringP("auth-jwt-required-issuer", "I", "", "when auth is enabled, sets the required JWTs issuer.")
	fJWTVerifier.StringP("auth-jwt-required-audience", "A", "", "when auth is enabled, sets the required JWTs audience.")
	fJWTVerifier.StringP("auth-jwt-principal-claim", "S", "", "when auth is enabled, sets the identity claim to use as the principal name to send to the policer.")
	fJWTVerifier.Bool("auth-jwks-insecure-skip-verify", false, "skip JWKS's server certificate validation. Don't do this.")

	fCORS.String("cors-origin", "*", "sets the valid HTTP Origin for CORS responses.")

	fAgentAuth.StringP("agent-token", "t", "", "JWT token to pass to the minibridge backend for agent identification.")
}
