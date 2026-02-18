package service

import (
	"errors"
	"fmt"

	"github.com/dcm-project/policy-manager/internal/opa"
)

// handleOPAError maps OPA errors to ServiceError types
func handleOPAError(err error, operation string) *ServiceError {
	if err == nil {
		return nil
	}

	// Check for specific OPA error types
	if errors.Is(err, opa.ErrPolicyNotFound) {
		return NewInternalError(
			fmt.Sprintf("Policy Rego code not found in OPA during %s", operation),
			"OPA does not have the Rego code for this policy",
			err,
		)
	}

	if errors.Is(err, opa.ErrInvalidRego) {
		return NewInvalidArgumentError(
			"Invalid Rego code",
			fmt.Sprintf("The Rego code contains syntax errors: %v", err),
		)
	}

	if errors.Is(err, opa.ErrOPAUnavailable) {
		return NewInternalError(
			fmt.Sprintf("OPA service unavailable during %s", operation),
			"Unable to communicate with the OPA service",
			err,
		)
	}

	if errors.Is(err, opa.ErrClientInternal) {
		return NewInternalError(
			fmt.Sprintf("OPA Client internal error during %s", operation),
			"An unexpected error occurred while communicating with OPA",
			err,
		)
	}

	// Generic OPA error
	return NewInternalError(
		fmt.Sprintf("OPA error during %s", operation),
		"An unexpected error occurred while communicating with OPA",
		err,
	)
}
