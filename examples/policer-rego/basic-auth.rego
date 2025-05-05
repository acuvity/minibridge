# This policy enforces a deny‐all default and, if the BASIC_AUTH env var is set,
# compares it to input.agent.password passed by the client as basic Authentication headers
# The request with the reason “invalid credentials” when they don’t match.
# To set BASIC_AUTH run export REGO_POLICY_RUNTIME_BASIC_AUTH="s3cr3t"
package main

import rego.v1

secret := opa.runtime().env.BASIC_AUTH

reasons contains "access denied" if {
	secret != ""
	input.agent.password != secret
}

default allow := false

allow if {
	count(reasons) == 0
}
