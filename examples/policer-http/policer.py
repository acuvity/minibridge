#!/bin/python
"""
This is an example implementation for a policer
"""

import json
import jwt
from termcolor import colored as c
from flask import Flask, request, Response

app = Flask(__name__)

FORBIDDEN_TOOLS = {
    "*": ["printEnv"],
    "bob@example.com": ["longRunningOperation"],
    "alice@example.com": [],
}


@app.route("/police", methods=["POST"])
def police():
    """handles /police"""

    req = request.get_json()
    agent = req["agent"]
    mcp = req["mcp"]

    print()
    print("---")
    print(c(f"Type: {req['type']}", "green" if req["type"] == "request" else "yellow"))
    print(c(f"Agent: {agent['userAgent']} {agent['remoteAddr']}", "blue"))
    print()
    print(c(f"{request.headers}", "dark_grey"))
    print(json.dumps(req, sort_keys=True, indent=4))

    # Check the agent token. Deny if not valid.
    # This example is meant to work with JWT issued
    # with the gen-test-tokens.sh located a the parent folder.
    try:
        claims = jwt.decode(
            agent["token"],
            "secret",
            algorithms=["HS256"],
            issuer="pki.example.com",
            audience="minibridge",
        )
    except Exception as e:
        print(c(f"DENIED: invalid token: {e}", "red"))
        return json.dumps({"allow": False, "reasons": [f"{e}"]})

    # This is an example of blanket policing. We deny access
    # to the tool/calls declared in FORBIDDEN_TOOLS
    if req["type"] == "request":
        if (
            mcp["method"] == "tools/call"
            and "name" in mcp["params"]
            and (
                mcp["params"]["name"] in FORBIDDEN_TOOLS["*"]
                or mcp["params"]["name"] in FORBIDDEN_TOOLS[claims["email"]]
            )
        ):
            dmsg = f"forbidden method call {mcp['params']['name']} {mcp['method']}"
            print(c(f"DENIED: {dmsg}", "red"))
            return json.dumps({"allow": False, "reasons": [dmsg]})

    # This is an example of redaction: If the
    # user is Bob, then we remove the tool named `longRunningOperation`.
    # from the response.
    if (
        req["type"] == "response"
        and "result" in req["mcp"]
        and claims["email"] == "bob@example.com"
    ):
        result = req["mcp"]["result"]
        if "tools" in result:
            result["tools"] = [
                cell
                for cell in result["tools"]
                if cell["name"] != "longRunningOperation"
            ]
            return Response(
                status=200, response=json.dumps({"allow": True, "mcp": mcp})
            )

    # otherwise we allow everything
    return Response(status=204)


app.run(port=5000)
