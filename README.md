# minibridge

Minibridge serves as a bridge between MCP servers and the outside world. It
functions as a backend-to-frontend connector, facilitating communication between
Agents and MCP servers. It securely exposes MCP servers to the internet and
optionally enables seamless integration with the Acuvity Platform.

Minibridge does not need to interpret the core MCP protocol, as it only handles
data streams. This design ensures forward compatibility with future changes to
the MCP protocol.

Currently, Minibridge is compatible with any compliant MCP server using protocol
version 2024-11-05. Support for version 2025-03-26 is in progress.

> Note: Minibridge is still under active development.

## All In One

Minibridge can act as a single gateway positioned in front of a standard
stdio-based MCP server.

To start everything as a single process, run:

    minibridge aio --listen :8000 -- npx -y @modelcontextprotocol/server-filesystem /tmp

This command launches both the frontend and backend within a single process,
which can be useful in certain scenarios.

The flow will look like the following:

```mermaid
flowchart LR
    agent -- http+sse --> minibridge
    minibridge -- stdio --> mcpserver
```

## Backend

Starting the backend launches an MCP server and exposes its API over a
WebSocket-based interface. TLS can be configured, with or without client
certificates, depending on your security requirements.

For example, to start a filesystem-based MCP server:

    minibridge backend -- npx -y @modelcontextprotocol/server-filesystem /tmp

You can now connect directly using a websocket client:

    wscat --connect ws://127.0.0.1:8080/ws

> NOTE: use the `wss` scheme if you have started minibridge backend with TLS.

> NOTE: Today, minibridge backend only supports MCP server over stdio.

The flow will look like the following:

```mermaid
flowchart LR
    agent -- websocket --> minibridge
    minibridge -- stdio --> mcpserver
```

## Frontend

While WebSockets address many of the limitations of plain POST+SSE, they are not
yet part of the official MCP protocol. To maintain backward compatibility with
existing agents, the frontend can expose a local interface using POST+SSE,
HTTP+STREAM (coming soon), or plain STDIO. It will then transparently forward
the data to the Minibridge backend over WebSockets and HTTPS.

### Stdio Frontend

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

### SSE Frontend

To start an SSE frontend:

    minibridge frontend --listen :8081 --backend wss://127.0.0.1:8000/ws

In this mode, a new WebSocket connection is established with the backend for
each incoming connection to the /sse endpoint. This preserves session state.
However, the WebSocket will not attempt to reconnect in this mode, and any
active streams will be terminated in the event of a network failure.

The flow will look like the following:

```mermaid
flowchart LR
    agent -- http+sse --> mb1[minibridge]
    mb1 -- websocket --> mb2[minibridge]
    mb2 -- stdio --> mcpserver
```

## Acuvity Integration

While Minibridge already offers advanced features such as strong client
authentication and native WebSocket support, it can be further enhanced through
integration with the Acuvity Platform. This integration unlocks a range of
additional capabilities, including:

* User authentication
* Role-based user authorization
* Input analysis and logging
* Full request tracing
* And more advanced policy-based controls

To enable integration with Acuvity, you must first have an Acuvity account. If
you don’t have one yet, you can [register here](https://console.acuvity.ai/signup).

Next, generate an App Token using the following command:

    acuctl api create apptoken --with.name my-mcp-server -n yourorg/apps -o json

You can then start Minibridge, using either the aio or backend subcommand, with
the following arguments:

    minibridge aio --policer-url https://policer.acme.com/police --policer-token $APPTOKEN

Once integrated, any command received by the backend is first forwarded to a
Policer for authentication and/or analysis.

If the request is denied, Minibridge will not forward it to the MCP server.
Instead, it will return a descriptive MCP error to the client, indicating why
the request was blocked.

Example:

    $ mcptools tools http://127.0.0.1:8000
    error: RPC error 451: request blocked: ForbiddenUser: I'm afraid you cannot do this, Dave
