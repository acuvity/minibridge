package main

import rego.v1

vclaims := claims if {
	[verified, _, claims] := io.jwt.decode_verify(
		input.agent.token,
		{
			"secret": "secret",
			"iss": "pki.example.com",
			"aud": "minibridge",
		},
	)

	print("token verified", verified, claims)
	verified == true
}

deny contains msg if {
	not vclaims
	msg := "You must provide a valid token"
}

deny contains msg if {
	input.mcp.method == "tools/call"
	input.mcp.params.name == "printEnv"
	vclaims.email != "john@example.com"
	msg := "You are not allowd to run printEnv"
}
