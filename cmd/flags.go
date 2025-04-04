package cmd

import (
	"github.com/spf13/pflag"
)

var (
	fTLSBackend  = pflag.NewFlagSet("tlsclient", pflag.ExitOnError)
	fTLSFrontend = pflag.NewFlagSet("tlsserver", pflag.ExitOnError)
	fProfiler    = pflag.NewFlagSet("profile", pflag.ExitOnError)
	fHealth      = pflag.NewFlagSet("health", pflag.ExitOnError)
)

func init() {

	fTLSFrontend.String("cert", "", "Path to the server certificate")
	fTLSFrontend.String("key", "", "Path to the key for the certificate")
	fTLSFrontend.String("key-pass", "", "Passphrase for the key")
	fTLSFrontend.String("client-ca", "", "Path to a CA to validate client connections")

	fTLSBackend.String("cert", "", "Path to the client certificate")
	fTLSBackend.String("key", "", "Path to the key for the certificate")
	fTLSBackend.String("key-pass", "", "Passphrase for the key")
	fTLSBackend.String("ca", "", "Path to a CA to validate server connections")
	fTLSBackend.Bool("insecure-skip-verify", false, "If set, don't validate server's CA. Do not do this.")

	fHealth.Bool("health-enable", false, "If set, start a health server for production deployments")
	fHealth.String("health-listen", ":8080", "Listen address of the health server")

	fProfiler.Bool("profiling-enable", false, "If set, enable profiling server")
	fProfiler.String("profiling-listen", ":6060", "Listen address of the health server")
}
