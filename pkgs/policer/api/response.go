package api

// Decision represents the Policer decision.
type Decision string

// Various values for Decision.
var (
	// DecisionAllow represents the Policer allowing the call.
	DecisionAllow Decision = "allow"

	// DecisionDeny represents the Policer refusing the call
	// because of an issue in the content of the MCP call.
	DecisionDeny Decision = "deny"

	// DecisionForbidden represents the Policer refusing the call
	// because of an authentication error.
	DecisionForbidden Decision = "forbidden"
)

// A Response is returned by the Policer.
type Response struct {

	// Decision of the Policer
	Decision Decision `json:"decision"`

	// Reasons for the decision. They are only set
	// when the Decision is now DecisionAllow.
	Reasons []string `json:"reasons,omitzero"`
}
