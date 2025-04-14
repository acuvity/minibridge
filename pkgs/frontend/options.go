package frontend

type sseCfg struct {
	sseEndpoint           string
	messagesEndpoint      string
	agentTokenPassthrough bool
	agentToken            string
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

// OptSSEAgentToken sets the token to send to the minibridge
// backend in order to authenticate the agent sending a request though
// the minibridge frontend.
func OptSSEAgentToken(tokenString string) OptSSE {
	return func(cfg *sseCfg) {
		cfg.agentToken = tokenString
	}
}

// OptSSEAgentTokenPassthrough decides if the HTTP request Authorization header
// should be passed as-is to the minibridge backend.
func OptSSEAgentTokenPassthrough(passthrough bool) OptSSE {
	return func(cfg *sseCfg) {
		cfg.agentTokenPassthrough = passthrough
	}
}

type stdioCfg struct {
	retry      bool
	agentToken string
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

// OptStdioAgentToken sets the token to send to the minibridge
// backend in order to authenticate the agent using the standard input.
func OptStioAgentToken(tokenString string) OptStdio {
	return func(cfg *stdioCfg) {
		cfg.agentToken = tokenString
	}
}
