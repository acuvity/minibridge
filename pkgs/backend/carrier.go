package backend

import (
	"go.acuvity.ai/minibridge/pkgs/mcp"
	"go.opentelemetry.io/otel/propagation"
)

var _ propagation.TextMapCarrier = metaCarrier{}

type metaCarrier struct {
	meta map[string]string
}

func newMCPMetaCarrier(call mcp.Message) metaCarrier {

	meta := map[string]string{}

	if call.Params != nil {
		if pmeta, ok := call.Params["_meta"].(map[string]any); ok {
			for k, v := range pmeta {
				if s, ok := v.(string); ok {
					meta[k] = s
				}
			}
		}
	}

	return metaCarrier{
		meta: meta,
	}
}

func (c metaCarrier) Get(key string) string {

	v, ok := c.meta[key]
	if !ok {
		return ""
	}

	return v
}

func (c metaCarrier) Set(key string, value string) {
	c.meta[key] = value
}

func (c metaCarrier) Keys() []string {

	out := make([]string, 0, len(c.meta))
	for k := range c.meta {
		out = append(out, k)
	}

	return out
}
