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

	initialized = false
)

func initSharedFlagSet() {

	if initialized {
		return
	}

	initialized = true

	fTLSServer.StringP("tls-server-cert", "c", "", "Path to the server certificate")
	fTLSServer.StringP("tls-server-key", "k", "", "Path to the key for the certificate")
	fTLSServer.StringP("tls-server-key-pass", "p", "", "Passphrase for the key")
	fTLSServer.String("tls-server-client-ca", "", "Path to a CA to validate client connections")

	fTLSClient.StringP("tls-client-cert", "C", "", "Path to the client certificate")
	fTLSClient.StringP("tls-client-key", "K", "", "Path to the key for the certificate")
	fTLSClient.StringP("tls-client-key-pass", "P", "", "Passphrase for the key")
	fTLSClient.String("tls-client-server-ca", "", "Path to a CA to validate server connections")
	fTLSClient.Bool("tls-insecure-skip-verify", false, "If set, don't validate server's CA. Do not do this.")

	fHealth.String("health-listen", ":8080", "Listen address of the health server")
	fHealth.Bool("health-enable", false, "If set, start a health server for production deployments")

	fProfiler.String("profiling-listen", ":6060", "Listen address of the health server")
	fProfiler.Bool("profiling-enable", false, "If set, enable profiling server")

	fPolicer.StringP("policer-url", "U", "", "Address of a Policer to send the traffic to for authentication and/or analysis")
	fPolicer.StringP("policer-token", "T", "", "Token to use to authenticate against the Policer")
	fPolicer.String("policer-ca", "", "CA to trust Policer server certificates")
	fPolicer.String("policer-insecure-skip-verify", "", "Do not validate Policer CA. Do not do this")

	fJWTVerifier.StringP("auth-jwks-url", "J", "", "If set, enables authentication and require JWT signed by a certificate in the given JWKS")
	fJWTVerifier.String("auth-jwks-ca", "", "If set, use the certificates in the provided PEM to trust the remote JWKS")
	fJWTVerifier.String("auth-jwt-cert", "", "If set, enables authentication and require JWT signed by the given certificate")
	fJWTVerifier.StringP("auth-jwt-required-issuer", "I", "", "Sets the required issuer in the JWT when auth is enabled")
	fJWTVerifier.StringP("auth-jwt-required-audience", "A", "", "Sets the required audience in the JWT when auth is enabled")
	fJWTVerifier.StringP("auth-jwt-principal-claim", "S", "", "Sets the identity claim to use to extract the principal user name when auth is enabled")
	fJWTVerifier.Bool("auth-jwks-insecure-skip-verify", false, "Don't validate the JWKS CA. Don't do this.")
}
