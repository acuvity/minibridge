package main

deny contains msg if {
	input.agent.token == ""
	msg := "You must provide a valid token"
}

deny contains msg if {
	input.mcp.method == "tools/call"
	input.mcp.params.name == "printEnv"
	msg := "You are not allowd to run printEnv"
}
