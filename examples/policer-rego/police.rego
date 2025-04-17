package main

import rego.v1

claims := x if {
	[verified, _, x] := io.jwt.decode_verify(
		input.agent.token,
		{
			"secret": "secret",
			"iss": "pki.example.com",
			"aud": "minibridge",
		},
	)
	verified == true
}

deny contains msg if {
	not claims
	msg := "You must provide a valid token"
}

deny contains msg if {
	input.mcp.method == "tools/call"
	input.mcp.params.name == "printEnv"
	claims.email != "alice@example.com"
	msg := "only alice can run printEnv"
}

mcp := x if {
	claims.email == "bob@example.com"

	x := json.patch(input.mcp, [{
		"op": "replace",
		"path": "/result/tools",
		"value": [x | x := input.mcp.result.tools[_]; x.name != "longRunningOperation"],
	}])
}
