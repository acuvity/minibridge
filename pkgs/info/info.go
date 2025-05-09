package info

type Info struct {
	OAuthAuthorize bool   `json:"oauthAuthorize"`
	OAuthRegister  bool   `json:"oauthRegister"`
	OAuthToken     bool   `json:"oauthToken"`
	OAuthMetadata  bool   `json:"oauthMetadata"`
	Type           string `json:"type"`
	Server         string `json:"server"`
}
