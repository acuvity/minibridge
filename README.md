# minibridge

> NOTE: this is a work in progress

Minibridge is a bridge between MCP servers and the rest of the world

It can be started in front of a remote MCP server to enable

- sessionless with websockets
- tls and mtls
- todo


## Server Example

To start a a file-server mcp using WS:

    minibridge server --listen 0.0.0.0:8000 -- npx -y @modelcontextprotocol/server-filesystem /tmp

To start a a file-server mcp using WSS:

    minibridge server --cert server-cert.pem --key server-key.pem -- npx -y @modelcontextprotocol/server-filesystem /tmp

To start a a file-server mcp using WSS and client certificates:

    minibridge server \
      --cert server-cert.pem \
      --key server-key.pem \
      --client-ca client-ca.pem -- npx -y @modelcontextprotocol/server-filesystem /tmp

## Client Example

Soon
