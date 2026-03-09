// Package engine handles engine API requests for policy evaluation.
package engine

import (
	"context"

	engineserver "github.com/dcm-project/policy-manager/internal/api/engine"
	"github.com/dcm-project/policy-manager/internal/service"
)

// Handler implements the engine API
type Handler struct {
	evaluationService service.EvaluationService
}

var _ engineserver.StrictServerInterface = (*Handler)(nil)

// NewHandler creates a new engine handler
func NewHandler(evaluationService service.EvaluationService) *Handler {
	return &Handler{
		evaluationService: evaluationService,
	}
}

// EvaluateRequest evaluates a service instance request against policies
func (h *Handler) EvaluateRequest(ctx context.Context, request engineserver.EvaluateRequestRequestObject) (engineserver.EvaluateRequestResponseObject, error) {
	// Convert API request to service request
	evaluationRequest, err := toServiceEvaluationRequest(request)
	if err != nil {
		return h.badRequest(err.Error()), nil
	}
	// Call evaluation service
	response, err := h.evaluationService.EvaluateRequest(ctx, evaluationRequest)
	if err != nil {
		return h.handleError(err), nil
	}

	// Map service response to API response
	return engineserver.EvaluateRequest200JSONResponse(toEngineEvaluationResponse(response)), nil
}
