package frontend

type sseCfg struct {
	sseEndpoint      string
	messagesEndpoint string
}

func newSSECfg() sseCfg {
	return sseCfg{
		sseEndpoint:      "/sse",
		messagesEndpoint: "/messages",
	}
}

// SSEOption are options that can be given to NewSSE().
type SSEOption func(*sseCfg)

// SSEOptionSSEEndpoint sets the sse endpoint
// where agents can connect to the response stream.
// Defaults to /sse
func SSEOptionSSEEndpoint(ep string) SSEOption {
	return func(cfg *sseCfg) {
		cfg.sseEndpoint = ep
	}
}

// SSEOptionMessageEndpoint sets the message endpoint
// where agents can post request.
// Defaults to /messages
func SSEOptionMessageEndpoint(ep string) SSEOption {
	return func(cfg *sseCfg) {
		cfg.messagesEndpoint = ep
	}
}
