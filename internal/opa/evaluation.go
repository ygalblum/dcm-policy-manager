package opa

// EvaluationResult represents the result from OPA evaluation
type EvaluationResult struct {
	Result  map[string]any // The policy decision
	Defined bool                    // Whether the policy made a decision
}

// PolicyDecision represents the expected output from OPA policies
type PolicyDecision struct {
	Rejected         bool                   `json:"rejected"`
	RejectionReason  string                 `json:"rejection_reason,omitempty"`
	OutputSpec       map[string]any `json:"output_spec,omitempty"`
	SelectedProvider string                 `json:"selected_provider,omitempty"`
}

// ParsePolicyDecision extracts a PolicyDecision from the OPA evaluation result
func ParsePolicyDecision(result map[string]any) *PolicyDecision {
	decision := &PolicyDecision{}

	if rejected, ok := result["rejected"].(bool); ok {
		decision.Rejected = rejected
	}

	if reason, ok := result["rejection_reason"].(string); ok {
		decision.RejectionReason = reason
	}

	if outputSpec, ok := result["output_spec"].(map[string]any); ok {
		decision.OutputSpec = outputSpec
	}

	if provider, ok := result["selected_provider"].(string); ok {
		decision.SelectedProvider = provider
	}

	return decision
}
