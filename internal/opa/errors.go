package opa

import (
	"errors"
	"fmt"
)

// Sentinel errors for OPA operations
var (
	// ErrPolicyNotFound indicates that the requested policy does not exist in OPA
	ErrPolicyNotFound = errors.New("policy not found in OPA")

	// ErrInvalidRego indicates that the Rego code is syntactically invalid
	ErrInvalidRego = errors.New("invalid Rego code")

	// ErrOPAUnavailable indicates that the OPA server is unreachable or returned an error
	ErrOPAUnavailable = errors.New("OPA service unavailable")

	// ErrClientInternal indicates that an internal error occurred in the client
	ErrClientInternal = errors.New("client internal error")
)

// OPAError represents an error response from the OPA server
type OPAError struct {
	Code    string   `json:"code,omitempty"`
	Message string   `json:"message,omitempty"`
	Errors  []string `json:"errors,omitempty"`
}

// Error implements the error interface
func (e *OPAError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("OPA error: %s", e.Message)
	}
	if len(e.Errors) > 0 {
		return fmt.Sprintf("OPA error: %s", e.Errors[0])
	}
	return "OPA error"
}
