# Roadmap

## In Progress

- [ ] Unit tests
- [ ] Wiki with tutorials on specific parts
- [ ] Support for 2025-03-26 (pr #21)

## Todo

- [ ] Advanced sandboxing when not running in containers (firejail/bubblewarp)
- [ ] Support for shared MCP server (when using a Policer)
- [ ] Add MTLS policer
- [ ] Add A3S Policer
- [ ] Add DScope Policer

## Done

- [x] Transport user information over the websocket channel
- [x] Support for user extraction to pass to the policer
- [x] Plug in prometheus metrics
- [x] Opentelemetry
- [x] Optimize communications between front/back in aio mode (use memconn)
