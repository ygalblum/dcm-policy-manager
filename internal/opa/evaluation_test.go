package opa

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParsePolicyDecision(t *testing.T) {
	tests := []struct {
		name     string
		result   map[string]interface{}
		expected *PolicyDecision
	}{
		{
			name: "approval with output spec",
			result: map[string]interface{}{
				"rejected": false,
				"output_spec": map[string]interface{}{
					"provider": "aws",
					"region":   "us-east-1",
				},
				"selected_provider": "aws",
			},
			expected: &PolicyDecision{
				Rejected: false,
				OutputSpec: map[string]interface{}{
					"provider": "aws",
					"region":   "us-east-1",
				},
				SelectedProvider: "aws",
			},
		},
		{
			name: "rejection with reason",
			result: map[string]interface{}{
				"rejected":         true,
				"rejection_reason": "Security policy violation",
			},
			expected: &PolicyDecision{
				Rejected:        true,
				RejectionReason: "Security policy violation",
			},
		},
		{
			name:   "empty result",
			result: map[string]interface{}{},
			expected: &PolicyDecision{
				Rejected: false,
			},
		},
		{
			name: "partial fields",
			result: map[string]interface{}{
				"rejected": false,
				"output_spec": map[string]interface{}{
					"instance_type": "t3.medium",
				},
			},
			expected: &PolicyDecision{
				Rejected: false,
				OutputSpec: map[string]interface{}{
					"instance_type": "t3.medium",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision := ParsePolicyDecision(tt.result)
			assert.Equal(t, tt.expected.Rejected, decision.Rejected)
			assert.Equal(t, tt.expected.RejectionReason, decision.RejectionReason)
			assert.Equal(t, tt.expected.OutputSpec, decision.OutputSpec)
			assert.Equal(t, tt.expected.SelectedProvider, decision.SelectedProvider)
		})
	}
}
