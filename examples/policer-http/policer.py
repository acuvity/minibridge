#!/bin/python
"""
This is an example implementation for a policer
"""

import json
from termcolor import colored
from flask import Flask, request, Response

app = Flask(__name__)

FORBIDDEN_METHODS = ["printEnv"]


@app.route("/police", methods=["POST"])
def police():
    """handles /police"""

    req = request.get_json()
    agent = req["agent"]
    mcp = req["mcp"]

    print()
    print("---")
    print(
        colored(f"Type: {req['type']}", "green" if req["type"] == "input" else "yellow")
    )
    print(colored(f"Agent: {agent['userAgent']} {agent['remoteAddr']}", "blue"))
    print()
    print(colored(f"{request.headers}", "dark_grey"))
    print(json.dumps(req, sort_keys=True, indent=4))

    # If there is no agent token, we refuse the call.
    # Of course, this is an example. IRL, you will validate that token.
    if req["agent"]["token"] == "":
        print(colored("DENIED: no token", "red"))
        return json.dumps(
            {
                "deny": ["you have not passed any token"],
            }
        )

    # As an example, we forbid user to use list_directory tools. Why? why not?
    if req["type"] == "input":
        if (
            mcp["method"] == "tools/call"
            and "name" in mcp["params"]
            and mcp["params"]["name"] in FORBIDDEN_METHODS
        ):
            denied_msg = (
                f"forbidden method call {mcp['params']['name']} {mcp['method']}"
            )
            print(colored(f"DENIED: {denied_msg}", "red"))
            return json.dumps({"deny": [denied_msg]})

    print(colored("ALLOWED", "green"))

    # otherwise we allow everything
    return Response(status=204)


app.run(port=5000)
