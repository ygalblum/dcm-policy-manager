package engine

import (
	engineserver "github.com/dcm-project/policy-manager/internal/api/engine"
	"github.com/dcm-project/policy-manager/internal/service"
)

func toServiceEvaluationRequest(request engineserver.EvaluateRequestRequestObject) *service.EvaluationRequest {
	return &service.EvaluationRequest{
		ServiceInstance: request.Body.ServiceInstance.Spec,
		RequestLabels:   extractRequestLabels(request.Body.ServiceInstance.Spec),
	}
}

func toEngineEvaluationResponse(response *service.EvaluationResponse) engineserver.EvaluateResponse {
	return engineserver.EvaluateResponse{
		EvaluatedServiceInstance: engineserver.ServiceInstance{
			Spec: response.EvaluatedServiceInstance,
		},
		SelectedProvider: response.SelectedProvider,
		Status:           engineserver.EvaluateResponseStatus(response.Status),
	}
}

// extractRequestLabels extracts labels from spec.metadata.labels
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
