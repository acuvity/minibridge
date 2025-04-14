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
	retry bool
}

func newStdioCfg() stdioCfg {
	return stdioCfg{
		retry: true,
	}
}

// OptStdio are options that can be given to NewStdio().
type OptStdio func(*stdioCfg)

// OptStdioRetry allows to control if the Stdio frontend
// should retry or not after a wbesocket connection failure.
func OptStdioRetry(retry bool) OptStdio {
	return func(cfg *stdioCfg) {
		cfg.retry = retry
	}
}
