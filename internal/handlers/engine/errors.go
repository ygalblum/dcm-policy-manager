package engine

import (
	"github.com/dcm-project/policy-manager/internal/service"
	engineserver "github.com/dcm-project/policy-manager/internal/api/engine"
)

// handleError maps service errors to HTTP responses
func (h *Handler) handleError(err error) engineserver.EvaluateRequestResponseObject {
	if serviceErr, ok := err.(*service.ServiceError); ok {
		switch serviceErr.Type {
		case service.ErrorTypeRejected:
			return h.rejected(serviceErr.Message, serviceErr.Detail)
		case service.ErrorTypePolicyConflict:
			return h.conflict(serviceErr.Message, serviceErr.Detail)
		case service.ErrorTypeInvalidArgument:
			return h.badRequest(serviceErr.Message)
		}
	}

	// Default to internal server error
	return h.internalError("Internal server error", "An unexpected error occurred")
}

// badRequest creates a 400 Bad Request response
func (h *Handler) badRequest(message string) engineserver.EvaluateRequestResponseObject {
	instance := "/api/v1alpha1/policies:evaluateRequest"
	return engineserver.EvaluateRequest400JSONResponse{
		BadRequestJSONResponse: engineserver.BadRequestJSONResponse{
			Type:     "about:blank",
			Status:   400,
			Title:    "Bad Request",
			Detail:   &message,
			Instance: &instance,
		},
	}
}

// rejected creates a 406 Not Acceptable response
func (h *Handler) rejected(title, detail string) engineserver.EvaluateRequestResponseObject {
	instance := "/api/v1alpha1/policies:evaluateRequest"
	return engineserver.EvaluateRequest406JSONResponse{
		RejectedJSONResponse: engineserver.RejectedJSONResponse{
			Type:     "about:blank",
			Status:   406,
			Title:    title,
			Detail:   &detail,
			Instance: &instance,
		},
	}
}

// conflict creates a 409 Conflict response
func (h *Handler) conflict(title, detail string) engineserver.EvaluateRequestResponseObject {
	instance := "/api/v1alpha1/policies:evaluateRequest"
	return engineserver.EvaluateRequest409JSONResponse{
		PolicyConflictJSONResponse: engineserver.PolicyConflictJSONResponse{
			Type:     "about:blank",
			Status:   409,
			Title:    title,
			Detail:   &detail,
			Instance: &instance,
		},
	}
}

// internalError creates a 500 Internal Server Error response
func (h *Handler) internalError(title, detail string) engineserver.EvaluateRequestResponseObject {
	instance := "/api/v1alpha1/policies:evaluateRequest"
	return engineserver.EvaluateRequest500JSONResponse{
		InternalServerErrorJSONResponse: engineserver.InternalServerErrorJSONResponse{
			Type:     "about:blank",
			Status:   500,
			Title:    title,
			Detail:   &detail,
			Instance: &instance,
		},
	}
}
