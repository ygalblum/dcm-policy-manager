package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dcm-project/policy-manager/api/v1alpha1"
	"github.com/dcm-project/policy-manager/internal/opa"
	"github.com/dcm-project/policy-manager/internal/store"
)

// EvaluationStatus represents the status of the evaluation
type EvaluationStatus string

const (
	EvaluationStatusApproved EvaluationStatus = "APPROVED"
	EvaluationStatusModified EvaluationStatus = "MODIFIED"
)

// EvaluationService defines the interface for policy evaluation
type EvaluationService interface {
	EvaluateRequest(ctx context.Context, req *EvaluationRequest) (*EvaluationResponse, error)
}

// EvaluationRequest represents a request for policy evaluation
type EvaluationRequest struct {
	ServiceInstance map[string]any
	RequestLabels   map[string]string
}

// EvaluationResponse represents the response from policy evaluation
type EvaluationResponse struct {
	EvaluatedServiceInstance map[string]any
	SelectedProvider         string
	Status                   EvaluationStatus
}

// evaluationService implements EvaluationService
type evaluationService struct {
	policyStore store.Policy
	opaClient   opa.Client
}

// NewEvaluationService creates a new evaluation service
func NewEvaluationService(policyStore store.Policy, opaClient opa.Client) EvaluationService {
	return &evaluationService{
		policyStore: policyStore,
		opaClient:   opaClient,
	}
}

// EvaluateRequest evaluates a service instance request against all applicable policies
func (s *evaluationService) EvaluateRequest(ctx context.Context, req *EvaluationRequest) (*EvaluationResponse, error) {
	// Retrieve all enabled policies, ordered by policy_type ASC, priority ASC
	// The default ordering is already policy_type ASC, priority ASC, id ASC
	result, err := s.policyStore.List(ctx, &store.PolicyListOptions{
		Filter: &store.PolicyFilter{
			Enabled: boolPtr(true),
		},
		PageSize: 1000, // Get all policies (assume we won't have more than 1000)
	})
	if err != nil {
		return nil, NewInternalError("Failed to retrieve policies", err.Error(), err)
	}

	policies := result.Policies

	// Initialize the current service instance spec (we'll modify this as we evaluate policies)
	currentSpec := deepCopyMap(req.ServiceInstance)
	originalSpec := deepCopyMap(req.ServiceInstance)

	// Initialize constraint context
	constraintCtx := NewConstraintContext()

	// Track selected provider across policies (starts unknown)
	selectedProvider := ""

	// Evaluate each policy sequentially
	for _, policy := range policies {
		// Filter by label selector
		if !MatchesLabelSelector(policy.LabelSelector, req.RequestLabels) {
			continue
		}

		// Evaluate the policy using the cached package name
		evalResult, err := s.opaClient.EvaluatePolicy(ctx, policy.PackageName, map[string]any{
			"spec":     currentSpec,
			"provider": selectedProvider,
		})
		if err != nil {
			return nil, NewInternalError(
				fmt.Sprintf("Failed to evaluate policy '%s'", policy.ID),
				err.Error(),
				err,
			)
		}

		// Skip if policy is undefined
		if !evalResult.Defined {
			continue
		}

		// Parse the policy decision
		decision := opa.ParsePolicyDecision(evalResult.Result)

		// Check for rejection
		if decision.Rejected {
			return nil, NewPolicyRejectedError(policy.ID, decision.RejectionReason)
		}

		// Check for output spec modifications
		if decision.OutputSpec != nil {
			// Check for constraint violations before applying changes
			violations := constraintCtx.CheckViolations(currentSpec, decision.OutputSpec)
			if len(violations) > 0 {
				// Get the first violation and return a conflict error
				firstViolation := violations[0]
				setByPolicy := constraintCtx.GetSetBy(firstViolation)
				return nil, NewPolicyConflictError(policy.ID, firstViolation, setByPolicy)
			}

			// Store current spec before modification
			beforeSpec := deepCopyMap(currentSpec)

			// Apply the modifications
			currentSpec = decision.OutputSpec

			// Mark changed fields as immutable
			constraintCtx.MarkChangedFields(beforeSpec, currentSpec, policy.ID)
		}

		// Update selected provider if specified
		if decision.SelectedProvider != "" {
			selectedProvider = decision.SelectedProvider
		}
	}

	// Determine status
	status := EvaluationStatusApproved
	if !mapsEqual(originalSpec, currentSpec) {
		status = EvaluationStatusModified
	}

	return &EvaluationResponse{
		EvaluatedServiceInstance: currentSpec,
		SelectedProvider:         selectedProvider,
		Status:                   status,
	}, nil
}

// extractRequestLabels extracts labels from the service instance spec at path spec.metadata.labels
func extractRequestLabels(spec map[string]any) map[string]string {
	if metadata, ok := spec["metadata"].(map[string]any); ok {
		if labels, ok := metadata["labels"].(map[string]any); ok {
			result := make(map[string]string)
			for k, v := range labels {
				if strVal, ok := v.(string); ok {
					result[k] = strVal
				}
			}
			return result
		}
	}
	return make(map[string]string)
}

// deepCopyMap creates a deep copy of a map
func deepCopyMap(m map[string]any) map[string]any {
	// Use JSON marshal/unmarshal for deep copy
	// This is simple but not the most efficient - could be optimized later
	bytes, err := json.Marshal(m)
	if err != nil {
		return m
	}

	var result map[string]any
	if err := json.Unmarshal(bytes, &result); err != nil {
		return m
	}

	return result
}

// mapsEqual checks if two maps are equal
func mapsEqual(a, b map[string]any) bool {
	// Use JSON comparison for simplicity
	aJSON, err := json.Marshal(a)
	if err != nil {
		return false
	}

	bJSON, err := json.Marshal(b)
	if err != nil {
		return false
	}

	return string(aJSON) == string(bJSON)
}

// boolPtr returns a pointer to a bool value
func boolPtr(b bool) *bool {
	return &b
}

// ConvertAPIToPolicyType converts an API policy type to internal policy type
func ConvertAPIToPolicyType(apiType v1alpha1.PolicyPolicyType) string {
	return string(apiType)
}
