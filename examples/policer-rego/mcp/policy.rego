package main

import rego.v1

# Pattern definitions
_covert_patterns := ["(?i)hide this"]

_redeaction_patterns := ["\\bbluefin\\b"]

_sensitive_patterns := ["\\btoken\\b"]

# deny reasons rules for tools/list results
reasons contains msg if {
	some tool in input.mcp.result.tools
	some pattern in _covert_patterns
	regex.match(pattern, tool.description)
	msg = sprintf("covert instruction in tool %v: %v", [tool.name, pattern])
}

reasons contains msg if {
	some tool in input.mcp.result.tools
	some pattern in _sensitive_patterns
	regex.match(pattern, tool.description)
	msg = sprintf("sensitive resource in tool %v: %v", [tool.name, pattern])
}

# deny reasons rules for tools/call
reasons contains msg if {
	input.mcp.method == "tools/call"
	some pattern in _covert_patterns
	regex.match(pattern, sprintf("%v", [input.mcp.params.arguments]))
	msg = sprintf("covert content in call args: %v", [pattern])
}

reasons contains msg if {
	input.mcp.method == "tools/call"
	some pattern in _sensitive_patterns
	regex.match(pattern, sprintf("%v", [input.mcp.params.arguments]))
	msg = sprintf("sensitive content in call args: %v", [pattern])
}

# Mutation: redact sensitive text in tool call responses
mcp := patched if {
	some idx, element in input.mcp.result.content
	element.type == "text"
	some pattern in _redeaction_patterns
	regex.match(pattern, element.text)
	patch = {
		"op": "replace",
		"path": sprintf("/result/content/%d/text", [idx]),
		"value": "[REDACTED]",
	}
	patched := json.patch(input.mcp, [patch])
}

# Allow only if no violations
allow if {
	count(reasons) == 0
}
