# minibridge

Minibridge is a bridge between MCP servers and the rest of the world. It
operates as a backend <-> frontend connector between Agents and MCP Servers. It
allows to expose MCP server securely over internet and allows seemless
integration to the Acuvity Plaform.

Minibridge does not need to understand the core MCP procotol as it only work
with data stream. This ensures forward compatibility with future changes on the
MCP procotol.

As of now, Minibridge should work with any compliant MCP server using protocol
version 2024-11-05. Support for 2025-03-26 is on the way.

> NOTE: minibridge is still under active development.

## All In One

Minibridge can operate as a single gateway to be placed in front of a stdio MCP
Server.

start everything as a single process:

minibridge aio --listen :8000 -- npx -y @modelcontextprotocol/server-filesystem /tmp

This will start both frontend and backend in a single process. This is useful in
some ocasions.

The flow will look like the following:

    agent -[http+sse]-> minibridge -[stdio]-> mcpserver

## Backend

Starting the backend will run an MCP server and expose its API over a
websocket-based API. It allows to configure TLS with or without client
certificates.

To start a a filesystem MCP server:

    minibridge backend -- npx -y @modelcontextprotocol/server-filesystem /tmp

You can now connect directly using a websocket client:

    wscat --connect ws://127.0.0.1:8080/ws

> NOTE: use wss scheme if you have started minibridge backend with TLS.

> NOTE: Today, minibridge backend only supports MCP server over stdio.

The flow will look like the following:

    agent -[websocket]-> minibridge -[stdio]-> mcpserver

## Frontend

While websockets remove a lot of issue plain POST+SSE brings, it is not part of
the MCP protocol yet. To be backward compatible with existing agents, frontend
can expose a local POST+SSE, HTTP+STREAM (soon) or plain STDIO to your agent,
and will deal with forwarding the data accordingly to the minibridge backend,
using websockets and HTTPS transparently.

### Stdio Frontend

To start an stdio frontend:

    minibridge frontend --backend wss://127.0.0.1:8000/ws

You can then send request to stdin and read responses to stdout. The frontend
will maintain a single WS connection to the backend, that will reconnect in case
of failures.

The flow will look like the following:

    agent -[stdio]-> minibridge -[websocket]-> minibridge -[stdio]-> mcpserver

### SSE Frontend

To start an SSE frontend:

    minibridge frontend --listen :8081 --backend wss://127.0.0.1:8000/ws

In this mode, a new websocket connection will be created to the backend for each
new connection received in the /sse endpoint. This maintains sessions. This
websocket wil not try to reconnect in that mode and active streams will be
shutdown in case of network failure.

The flow will look like the following:

    agent -[http+sse]-> minibridge -[websocket]-> minibridge -[stdio]-> mcpserver

## Acuvity Integration

While minibridge by itself already brings serious features like strong client
authentication, websocket by itself, it can be used with the Acuvity platform.
With such an integration the following features will be available:

* bearer authn
* rego based authz
* analysis and logging of inputs
* tracing of the requests
* redactions of sensitive data
* much more
