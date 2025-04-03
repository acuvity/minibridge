# minibridge

> NOTE: this is a work in progress

Minibridge is a bridge between MCP servers and the rest of the world. It
operates as a backend <-> frontend connector between Agents and MCP Servers.
It allows to expose MCP server securely over internet and allows seemless
integration to the Acuvity



## Backend Example

To start a a file-server mcp using WS:

    minibridge backend --listen 0.0.0.0:8000 -- npx -y @modelcontextprotocol/server-filesystem /tmp

To start a a file-server mcp using WSS:

    minibridge backend --cert server-cert.pem --key server-key.pem -- npx -y @modelcontextprotocol/server-filesystem /tmp

To start a a file-server mcp using WSS and client certificates:

    minibridge backend \
      --cert server-cert.pem \
      --key server-key.pem \
      --client-ca client-ca.pem -- npx -y @modelcontextprotocol/server-filesystem /tmp

## Frontend Example

While websockets remove a lot of issue plain POST+SSE brings, it is not part of
the MCP protocol yet. To be backward compatible with existing agent, frontend
can expose a local POST+SSE, HTTP+STREAM or plain STDIO to your agent, and will
deal with forwarding the data accordingly to the minibridge backend, using
websockets and HTTPS transparently.

### Stdio Frontend

To start an stdio frontend:

    minibridge frontend --backend wss://127.0.0.1:8000/ws

You can then send request to stdin and read responses to stdout. The frontend
will maintain a single WS connection to the backend, that will reconnect in case
of failures.


### SSE Frontend

To start an SSE frontend:

    minibridge frontend --listen :8081 --backend wss://127.0.0.1:8000/ws

In this mode, a new websocket connection will be created to the backend for each
new connection received in the /sse endpoint. This maintains sessions. This
websocket wil not try to reconnect in that mode and active streams will be
shutdown in case of network failure.
