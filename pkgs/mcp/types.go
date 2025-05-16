package mcp

type Tools []Tool
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"inputSchema,omitempty"`
}

type Prompts []*Prompt
type Prompt struct {
	Name        string          `json:"name,omitempty"`
	Description string          `json:"description,omitempty"`
	Arguments   PromptArguments `json:"arguments,omitempty"`
}

type PromptArguments []PromptArgument
type PromptArgument struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

type Resources []Resource
type Resource struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
	Text        string `json:"text,omitempty"`
	Blob        string `json:"blob,omitempty"`
	URI         string `json:"uri,omitempty"`
}

type ResourceTemplates []ResourceTemplate
type ResourceTemplate struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	URITemplate string `json:"uriTemplate,omitempty"`
}
