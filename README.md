# minibridge

Minibridge serves as a bridge between MCP servers and the outside world. It
functions as a backend-to-frontend connector, facilitating communication between
Agents and MCP servers. It allows to securely exposes MCP servers to the internet and
optionally enables seamless integration with a generic policing service for
agent authentication and content analysis.

Minibridge does not need to interpret the core MCP protocol, as it only handles
data streams. This design ensures forward compatibility with future changes to
the MCP protocol.

Currently, Minibridge is compatible with any compliant MCP server using protocol
version 2024-11-05. Support for version 2025-03-26 is in progress.

> Note: Minibridge is still under active development.


## Table of Content

<!-- vim-markdown-toc GFM -->

* [All In One](#all-in-one)
* [Backend](#backend)
* [Frontend](#frontend)
  * [Stdio](#stdio)
  * [HTTP+SSE](#httpsse)
* [Policer and Authentication](#policer-and-authentication)
  * [Policer](#policer)
  * [Authentication](#authentication)
* [Todos](#todos)

<!-- vim-markdown-toc -->

## All In One

Minibridge can act as a single gateway positioned in front of a standard
stdio-based MCP server.

To start everything as a single process, run:

    minibridge aio --listen :8000 -- npx -y @modelcontextprotocol/server-filesystem /tmp

This command launches both the frontend and backend within a single process,
which can be useful in certain scenarios.

You can connect directly using an HTTP client:

    $ curl http://127.0.0.1:8000/sse
    event: endpoint
    data: /message?sessionId=UID

    $ curl http://127.0.0.1:8000/message?sessionId=UID \
      -X POST \
      -d '{"jsonrpc":"2.0","id":2,"method":"tools/list"}'

The flow will look like the following:

```mermaid
flowchart LR
    agent -- http+sse --> minibridge
    minibridge -- stdio --> mcpserver
```

In order to secure the connections, you need to enable HTTPS for incoming
connections:

    minibridge aio --listen :8443 \
      --tls-server-cert ./server-cert.pem \
      --tls-server-key ./server-key.pem \
      --tls-server-client-ca ./clients-ca.pem \
      -- npx -y @modelcontextprotocol/server-filesystem /tmp

This enables HTTPS and with `--tls-server-client-ca`, it requires the clients to
send a certificate signed by that client CA.

You can now connect directly using an HTTP client:

    $ curl https://127.0.0.1:8443/sse \
      --cacert ./server-cert.pem --cert ./client-cert.pem --key ./client-key.pem
    event: endpoint
    data: /message?sessionId=UID

    $ curl https://127.0.0.1:8443/message?sessionId=UID \
      --cacert ./server-cert.pem --cert ./client-cert.pem --key ./client-key.pem \
      -X POST \
      -d '{"jsonrpc":"2.0","id":2,"method":"tools/list"}'

## Backend

Starting the backend launches an MCP server and exposes its API over a
WebSocket-based interface. TLS can be configured, with or without client
certificates, depending on your security requirements.

For example, to start a filesystem-based MCP server:

    minibridge backend -- npx -y @modelcontextprotocol/server-filesystem /tmp

You can now connect directly using a websocket client:

    wscat --connect ws://127.0.0.1:8000/ws

> NOTE: use the `wss` scheme if you have started minibridge backend with TLS.

> NOTE: Today, minibridge backend only supports MCP server over stdio.

The flow will look like the following:

```mermaid
flowchart LR
    agent -- websocket --> minibridge
    minibridge -- stdio --> mcpserver
```

In order to secure the connections, you need to enable HTTPS for incoming
connections:

    minibridge backend --listen :8443 \
      --tls-server-cert ./backend-server-cert.pem \
      --tls-server-key ./backend-server-key.pem \
      --tls-server-client-ca ./clients-ca.pem \
      -- npx -y @modelcontextprotocol/server-filesystem /tmp

This enables HTTPS and with `--tls-server-client-ca`, it requires the clients to
send a certificate signed by that client CA. You can now connect using:

    wscat --connect wss://127.0.0.1:8443/ws \
      --ca ./server-cert.pem \
      --cert ./client-cert.pem

## Frontend

While WebSockets address many of the limitations of plain POST+SSE, they are not
yet part of the official MCP protocol. To maintain backward compatibility with
existing agents, the frontend can expose a local interface using POST+SSE,
HTTP+STREAM (coming soon), or plain STDIO. It will then transparently forward
the data to the Minibridge backend over WebSockets and HTTPS.

### Stdio

To start an stdio frontend:

    minibridge frontend --backend wss://127.0.0.1:8000/ws

You can then send requests via stdin and read responses from stdout. The
frontend maintains a single WebSocket connection to the backend and will
automatically reconnect in case of failures.

The flow will look like the following:

```mermaid
flowchart LR
    agent -- stdio --> mb1[minibridge]
    mb1 -- websocket --> mb2[minibridge]
    mb2 -- stdio --> mcpserver
```

### HTTP+SSE

To start an SSE frontend:

    minibridge frontend --listen :8081 --backend wss://127.0.0.1:8000/ws

In this mode, a new WebSocket connection is established with the backend for
each incoming connection to the /sse endpoint. This preserves session state.
However, the WebSocket will not attempt to reconnect in this mode, and any
active streams will be terminated in the event of a network failure.

You can connect directly using an HTTP client:

    $ curl http://127.0.0.1:8001/sse
    event: endpoint
    data: /message?sessionId=UID

    $ curl http://127.0.0.1:8001/message?sessionId=UID \
      -X POST \
      -d '{"jsonrpc":"2.0","id":2,"method":"tools/list"}'

The flow will look like the following:

```mermaid
flowchart LR
    agent -- http+sse --> mb1[minibridge]
    mb1 -- websocket --> mb2[minibridge]
    mb2 -- stdio --> mcpserver
```

In order to secure the connections, you need to enable HTTPS for incoming
connections:

    minibridge frontend --listen :8444 \
      --backend wss://127.0.0.1:8000/ws
      --tls-server-cert ./server-cert.pem \
      --tls-server-key ./server-key.pem \
      --tls-server-client-ca client-ca.pem ]\
      --tls-client-cert ./client-cert.pem \
      --tls-client-key ./client-key.pem \
      --tls-client-backend-ca ./backend-server-cert.pem

This enables HTTPS and with `--tls-server-client-ca`, it requires the clients to
send a certificate signed by that client CA. It also make the front end to
authenticate to the backend using the provided client certificate (MTLS).

You can now connect directly using an HTTP client:

    $ curl https://127.0.0.1:8444/sse \
      --cacert ./server-cert.pem --cert ./client-cert.pem --key ./client-key.pem
    event: endpoint
    data: /message?sessionId=UID

    $ curl https://127.0.0.1:8444/message?sessionId=UID \
      --cacert ./server-cert.pem --cert ./client-cert.pem --key ./client-key.pem \
      -X POST \
      -d '{"jsonrpc":"2.0","id":2,"method":"tools/list"}'

## Policer and Authentication

While Minibridge already offers advanced features such as strong client
authentication and native WebSocket support, it can be further enhanced through
integration with a Policer. A Policer is responsible for:

* Authorization
* Input analysis and logging
* Full request tracing
* And more advanced policy-based controls

### Policer

The Policer, if set, will be called and passed various information so it can
make a decision on what to do with the request, based on the user who initiated
the request and the content of the request.

You can then start Minibridge, using either the aio or backend subcommand, with
the following arguments:

    minibridge aio --policer-url https://policer.acme.com/police --policer-token $PTOKEN

Once integrated, any command from the user or response from the MCP Server
received by the backend is first passed to the Policer for authentication and/or
analysis.

The Policer receives a `POST` request at the `--policer-url` endpoint in the
following format:

```json
{
  "type":"Input"
  "messages": ["{\"jsonrpc\":\"2.0\",\"method\":\"tools/list\",\"id\":1}"],
  "user": {
    "name": "joe",
    "claims": ["email=joe@acme.com", "group=mcp-users"]
  }
}
```

> NOTE: for a response from the MCP Server, the `type` will be set to `Output`
and no user will be passed.

The Policer must respond with an HTTP status code `200 OK` if the request passes
the policy checks. Any other status code will be treated as a failure, and the
request will be blocked.

For a policy decision that permits the request:

```json
{
  "decision": "Allow"
}
```

For a policy result that denies the request:

```json
{
  "decision": "Deny",
  "reasons": ["You are not allowed to list the tools"]
}
```

If the request is denied (or the Policer does not return `200 OK`), Minibridge
will not forward it to the MCP server. Instead, it will return a descriptive MCP
error to the client, indicating why the request was blocked.

Example:

    $ mcptools tools http://127.0.0.1:8000
    error: RPC error 451: request blocked: ForbiddenUser: You are not allowed to list the tools

### Authentication

In order for the policer to identify a particular user (instead of using the
identity contained into `--policer-token`), Minibridge supports JWT verification
and information extraction.

To start minibridge with auth enabled:

    minibridge backend \
      --policer-url https://policer.acme.com/police --policer-token $PTOKEN \
      --auth-jwks-url https://myidp.com/.well-known/jwks.json \
      --auth-jwt-principal-claim email

This makes the backend (or AIO backend) requires clients to pass a JWT signed by
a key contained in the JWKS available at
`https://myidp.com/.well-known/jwks.json`.

Instead of a JWKS, you can use local certificates contained into a PEM file by
passing `--auth-jwt-cert ./jwt.pem` instead of setting `--auth-jwks-url`.

You can also require a specific issuer and audience with the flags
`--auth-jwt-required-issuer` and `--auth-jwt-required-audience`

The principal claim is used to extract a particular key from the JWT's
`identity` property and use it as the principal name when sending policing
request to the policer.

Now that the backend requires a JWT, you have 2 options for the frontend to
forward the user JWT:

* Either you set a global token for the frontend using the flag `--agent-token`:

      minibridge frontend -l :8001 --agent-token $TOKEN

* Or you forward the incoming `Authorization` header as-is to the backend with
`--agent-token-passthrough`:

      minibridge frontend -l :8001 --agent-token-passthrough

## Todos

Minibridge is still missing the following features:

- [ ] Unit tests
- [x] Transport user information over the websocket channel
- [x] Support for user extraction to pass to the policer
- [ ] Optimize communications between front/back in aio mode
- [ ] Plug in prometheus metrics
- [ ] Support connecting to an MCP server using HTTP
