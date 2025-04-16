package api

// A Response is returned by the Policer.
type Response struct {

	// Deny contains reasons for denying the request.
	Deny []string `json:"deny,omitzero"`
}
