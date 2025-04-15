#!/bin/python

import json
from flask import Flask, request

app = Flask(__name__)

@app.route("/police", methods=['POST'])
def police():
    req = request.get_json()


    print("---")
    print("Type: %s" % req['type'])
    print("Headers: %s" % request.headers)
    print(json.dumps(req, sort_keys=True, indent=4))

    # If there is no agent token, we refuse the call.
    # Of course, this is an example. IRL, you will validate that token.
    if req['token'] == "":
        print("REJECTED: no token")
        return json.dumps({
            "decision": "deny",
            "reasons": ["you have not passed any token"],
        })

    # As an example, we forbid user to use list_directory tools. Why? why not?
    if req['type'] == 'input':
        mcp = req['mcp']
        if mcp['method'] == "tools/call" and mcp['params']['name'] == "list_directory":
            return json.dumps({
                "decision": "deny",
                "reasons": ["you cannot list directories"],
            })

    # otherwise we allow everything
    return json.dumps({"decision": "allow"})

app.run(port=5000)
