package cmd

import (
	"github.com/spf13/pflag"
)

var (
	fTLSClient = pflag.NewFlagSet("tlsclient", pflag.ExitOnError)
	fTLSServer = pflag.NewFlagSet("tlsserver", pflag.ExitOnError)
	fProfiler  = pflag.NewFlagSet("profile", pflag.ExitOnError)
	fHealth    = pflag.NewFlagSet("health", pflag.ExitOnError)
	fPolicer   = pflag.NewFlagSet("policer", pflag.ExitOnError)
	fCORS      = pflag.NewFlagSet("cors", pflag.ExitOnError)
	fAgentAuth = pflag.NewFlagSet("agentauth", pflag.ExitOnError)

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
	fTLSServer.String("tls-server-client-ca", "", "path to a CA to require and validate incoming client certificates.")

	fTLSClient.StringP("tls-client-cert", "C", "", "path to the client certificate to authenticate against the minibridge backend.")
	fTLSClient.StringP("tls-client-key", "K", "", "path to the key for the client certificate.")
	fTLSClient.StringP("tls-client-key-pass", "P", "", "passphrase for the client certificate key.")
	fTLSClient.String("tls-client-backend-ca", "", "path to a CA to validate the minibridge backend server certificates.")
	fTLSClient.Bool("tls-client-insecure-skip-verify", false, "skip backend's server certificates validation. INSECURE.")

	fHealth.String("health-listen", ":8080", "listen address of the health server.")
	fHealth.Bool("health-enable", false, "enables health server.")

	fProfiler.String("profiling-listen", ":6060", "listen address of the health server.")
	fProfiler.Bool("profiling-enable", false, "enables profiling server.")

	fPolicer.StringP("policer-type", "P", "", "type of policer to use. use --policer-list to list all of them.")
	fPolicer.String("policer-rego-policy", "", "path to a rego policy file for the rego policer.")
	fPolicer.String("policer-http-url", "", "URL of the HTTP policer to POST agent policing requests.")
	fPolicer.String("policer-http-token", "", "token to use to authenticate against the HTTP policer.")
	fPolicer.String("policer-http-ca", "", "path to a CA to validate the policer server certificates.")
	fPolicer.Bool("policer-http-insecure-skip-verify", false, "skip policer's server certificates validation. INSECURE.")

	fCORS.String("cors-origin", "*", "sets the valid HTTP Origin for CORS responses.")

	fAgentAuth.StringP("agent-token", "t", "", "JWT token to pass to the minibridge backend for agent identification.")
}
