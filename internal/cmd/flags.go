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

	fTLSServer.String("cert", "", "Path to the server certificate")
	fTLSServer.String("key", "", "Path to the key for the certificate")
	fTLSServer.String("key-pass", "", "Passphrase for the key")
	fTLSServer.String("client-ca", "", "Path to a CA to validate client connections")

	fTLSClient.String("client-cert", "", "Path to the client certificate")
	fTLSClient.String("client-key", "", "Path to the key for the certificate")
	fTLSClient.String("client-key-pass", "", "Passphrase for the key")
	fTLSClient.String("server-ca", "", "Path to a CA to validate server connections")
	fTLSClient.Bool("insecure-skip-verify", false, "If set, don't validate server's CA. Do not do this.")

	fHealth.Bool("health-enable", false, "If set, start a health server for production deployments")
	fHealth.String("health-listen", ":8080", "Listen address of the health server")

	fProfiler.Bool("profiling-enable", false, "If set, enable profiling server")
	fProfiler.String("profiling-listen", ":6060", "Listen address of the health server")

	fPolicer.String("policer-url", "", "Address of a Policer to send the traffic to for authentication and/or analysis")
	fPolicer.String("policer-token", "", "Token to use to authenticate against the Policer")
	fPolicer.String("policer-ca", "", "CA to trust Policer server certificates")
	fPolicer.String("policer-insecure-skip-verify", "", "Do not validate Policer CA. Do not do this")

	fJWTVerifier.String("jwks-url", "", "If set, enables authentication and require JWT signed by a certificate in the given JWKS")
	fJWTVerifier.String("jwks-ca", "", "If set, use the certificates in the provided PEM to trust the remote JWKS")
	fJWTVerifier.Bool("jwks-insecure-skip-verify", false, "Don't validate the JWKS CA. Don't do this.")
	fJWTVerifier.String("jwt-cert", "", "If set, enables authentication and require JWT signed by the given certificate")
	fJWTVerifier.String("jwt-required-issuer", "", "Sets the required issuer in the JWT when auth is enabled")
	fJWTVerifier.String("jwt-required-audience", "", "Sets the required audience in the JWT when auth is enabled")
	fJWTVerifier.String("jwt-principal-claim", "", "Sets the identity claim to use to extract the principal user name when auth is enabled")
}
