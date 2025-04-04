package cmd

import (
	"github.com/spf13/pflag"
)

var (
	fTLSClient = pflag.NewFlagSet("tlsclient", pflag.ExitOnError)
	fTLSServer = pflag.NewFlagSet("tlsserver", pflag.ExitOnError)
	fProfiler  = pflag.NewFlagSet("profile", pflag.ExitOnError)
	fHealth    = pflag.NewFlagSet("health", pflag.ExitOnError)
)

func init() {

	fTLSServer.String("cert", "", "Path to the server certificate")
	fTLSServer.String("key", "", "Path to the key for the certificate")
	fTLSServer.String("key-pass", "", "Passphrase for the key")
	fTLSClient.String("client-ca", "", "Path to a CA to validate client connections")

	fTLSClient.String("cert", "", "Path to the client certificate")
	fTLSClient.String("key", "", "Path to the key for the certificate")
	fTLSClient.String("key-pass", "", "Passphrase for the key")
	fTLSClient.String("ca", "", "Path to a CA to validate server connections")
	fTLSClient.Bool("insecure-skip-verify", false, "If set, don't validate server's CA. Do not do this.")

	fHealth.Bool("health-enable", false, "If set, start a health server for production deployments")
	fHealth.String("health-listen", ":8080", "Listen address of the health server")

	fProfiler.Bool("profiling-enable", false, "If set, enable profiling server")
	fProfiler.String("profiling-listen", ":6060", "Listen address of the health server")
}
