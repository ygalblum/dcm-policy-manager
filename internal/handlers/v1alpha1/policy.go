// Package v1alpha1 handles v1alpha1 API requests for policy CRUD operations.
package v1alpha1

import (
	"context"

	"github.com/dcm-project/policy-manager/api/v1alpha1"
	"github.com/dcm-project/policy-manager/internal/api/server"
	"github.com/dcm-project/policy-manager/internal/service"
)

type PolicyHandler struct {
	service service.PolicyService
}

// Ensure PolicyHandler implements StrictServerInterface
var _ server.StrictServerInterface = (*PolicyHandler)(nil)

func NewPolicyHandler(service service.PolicyService) *PolicyHandler {
	return &PolicyHandler{
		service: service,
	}
}

// GetHealth handles health check requests.
func (h *PolicyHandler) GetHealth(_ context.Context, _ server.GetHealthRequestObject) (server.GetHealthResponseObject, error) {
	status := "ok"
	path := "health"
	return server.GetHealth200JSONResponse{
		Status: status,
		Path:   &path,
	}, nil
}

// CreatePolicy handles creating a new policy resource.
func (h *PolicyHandler) CreatePolicy(ctx context.Context, request server.CreatePolicyRequestObject) (server.CreatePolicyResponseObject, error) {
	if request.Body == nil {
		return server.CreatePolicy400JSONResponse{
			BadRequestJSONResponse: badRequestResponse(buildErrorResponse(
				400,
				v1alpha1.INVALIDARGUMENT,
				"Invalid request body",
				strPtr("Request body is required"),
			)),
		}, nil
	}

	// Convert server.Policy to v1alpha1.Policy
	v1Alpha1Policy := policyServerToV1Alpha1(*request.Body)

	// Call service to create policy
	created, err := h.service.CreatePolicy(ctx, v1Alpha1Policy, request.Params.Id)
	if err != nil {
		return h.handleCreatePolicyError(err, request), nil
	}

	// Convert back to server.Policy
	return server.CreatePolicy201JSONResponse{
		Body: policyV1Alpha1ToServer(*created),
	}, nil
}

// GetPolicy handles retrieving a single policy by ID.
func (h *PolicyHandler) GetPolicy(ctx context.Context, request server.GetPolicyRequestObject) (server.GetPolicyResponseObject, error) {
	// Call service to get policy
	policy, err := h.service.GetPolicy(ctx, request.PolicyId)
	if err != nil {
		return h.handleGetPolicyError(err, request), nil
	}

	return server.GetPolicy200JSONResponse(policyV1Alpha1ToServer(*policy)), nil
}

// ListPolicies handles listing policies with optional filtering and pagination.
func (h *PolicyHandler) ListPolicies(ctx context.Context, request server.ListPoliciesRequestObject) (server.ListPoliciesResponseObject, error) {
	// Extract parameters with defaults handled by service
	result, err := h.service.ListPolicies(
		ctx,
		request.Params.Filter,
		request.Params.OrderBy,
		request.Params.PageToken,
		request.Params.MaxPageSize,
	)
	if err != nil {
		return h.handleListPoliciesError(err, request), nil
	}

	return server.ListPolicies200JSONResponse(listResponseV1Alpha1ToServer(*result)), nil
}

// UpdatePolicy handles updating an existing policy resource.
func (h *PolicyHandler) UpdatePolicy(ctx context.Context, request server.UpdatePolicyRequestObject) (server.UpdatePolicyResponseObject, error) {
	if request.Body == nil {
		return server.UpdatePolicy400JSONResponse{
			BadRequestJSONResponse: badRequestResponse(buildErrorResponse(
				400,
				v1alpha1.INVALIDARGUMENT,
				"Invalid request body",
				strPtr("Request body is required"),
			)),
		}, nil
	}

	// Convert server Policy (PATCH body) to api/v1alpha1 Policy
	patch := policyServerToV1Alpha1(*request.Body)

	// Call service to update policy (merge patch onto existing)
	updated, err := h.service.UpdatePolicy(ctx, request.PolicyId, &patch)
	if err != nil {
		return h.handleUpdatePolicyError(err, request), nil
	}

	return server.UpdatePolicy200JSONResponse(policyV1Alpha1ToServer(*updated)), nil
}

// DeletePolicy handles deleting a policy by ID.
func (h *PolicyHandler) DeletePolicy(ctx context.Context, request server.DeletePolicyRequestObject) (server.DeletePolicyResponseObject, error) {
	// Call service to delete policy
	err := h.service.DeletePolicy(ctx, request.PolicyId)
	if err != nil {
		return h.handleDeletePolicyError(err, request), nil
	}

	return server.DeletePolicy204Response{}, nil
}
