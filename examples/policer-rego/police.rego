package main

import rego.v1

# the call is allowed is we don't have any reason to deny it.
allow if {
	count(reasons) == 0
}

# verifies the claims from the JWT of the agent.
claims := x if {
	[verified, _, x] := io.jwt.decode_verify(
		input.agent.password,
		{
			"secret": "secret",
			"iss": "pki.example.com",
			"aud": "minibridge",
		},
	)
	verified == true
}

# if we have no claims, we add a deny reason.
reasons contains msg if {
	not claims
	msg := "You must provide a valid token"
}

# if the call is a tools/call and the name
# is printEnv and the user is not Alice,
# we add a deny reason.
reasons contains msg if {
	input.mcp.method == "tools/call"
	input.mcp.params.name == "printEnv"
	claims.email != "alice@example.com"
	msg := "only alice can run printEnv"
}

# if the call is a tools/call and the name
# is longRunningOperation and the user is Bob,
# we add a deny reason.
reasons contains msg if {
	input.mcp.method == "tools/call"
	claims.email == "bob@example.com"
	input.mcp.params.name == "longRunningOperation"
	msg := "bob cannot run longRunningOperation"
}

# if the call is a tools/call and the request is from Bob, we remove the
# longRunningOperation from the response. If Bob still tries to call that tool,
# it will be denied by the rule above. This allows the agent to not loose time
# trying a tool that will be denied anyways
mcp := x if {
	input.mcp.result.tools
	claims.email == "bob@example.com"

	x := json.patch(input.mcp, [{
		"op": "replace",
		"path": "/result/tools",
		"value": [x | x := input.mcp.result.tools[_]; x.name != "longRunningOperation"],
	}])
}
