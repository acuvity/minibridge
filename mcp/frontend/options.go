package frontend

type sseCfg struct {
	sseEndpoint      string
	messagesEndpoint string
}

func newSSECfg() sseCfg {
	return sseCfg{
		sseEndpoint:      "/sse",
		messagesEndpoint: "/message",
	}
}

// OptSSE are options that can be given to NewSSE().
type OptSSE func(*sseCfg)

// OptSSEStreamEndpoint sets the sse endpoint
// where agents can connect to the response stream.
// Defaults to /sse
func OptSSEStreamEndpoint(ep string) OptSSE {
	return func(cfg *sseCfg) {
		cfg.sseEndpoint = ep
	}
}

// OptSSEMessageEndpoint sets the message endpoint
// where agents can post request.
// Defaults to /messages
func OptSSEMessageEndpoint(ep string) OptSSE {
	return func(cfg *sseCfg) {
		cfg.messagesEndpoint = ep
	}
}

type stdioCfg struct {
}

func newStdioCfg() stdioCfg {
	return stdioCfg{}
}

// OptStdio are options that can be given to NewStdio().
type OptStdio func(*stdioCfg)
