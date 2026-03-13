// Package opa provides an HTTP client for managing and evaluating policies in Open Policy Agent.
package opa

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/dcm-project/policy-manager/internal/logging"
)

const (
	opaEndpointPattern = "%s/v1/policies/%s"
)

// Client defines the interface for interacting with OPA
type Client interface {
	// StorePolicy stores or updates a policy in OPA
	// Returns ErrInvalidRego if the Rego code is invalid
	// Returns ErrOPAUnavailable if OPA is unreachable
	StorePolicy(ctx context.Context, policyID string, regoCode string) error

	// GetPolicy retrieves a policy from OPA
	// Returns ErrPolicyNotFound if the policy doesn't exist
	// Returns ErrOPAUnavailable if OPA is unreachable
	GetPolicy(ctx context.Context, policyID string) (string, error)

	// DeletePolicy removes a policy from OPA
	// Returns nil even if the policy doesn't exist (idempotent)
	// Returns ErrOPAUnavailable if OPA is unreachable
	DeletePolicy(ctx context.Context, policyID string) error

	// EvaluatePolicy evaluates input against a policy by package name
	// Returns ErrPolicyNotFound if the policy doesn't exist
	// Returns ErrOPAUnavailable if OPA is unreachable
	EvaluatePolicy(ctx context.Context, packageName string, input map[string]any) (*EvaluationResult, error)
}

// HTTPClient implements the Client interface using HTTP requests to OPA
type HTTPClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new OPA HTTP client
func NewClient(baseURL string, timeout time.Duration) Client {
	return &HTTPClient{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// StorePolicy stores or updates a policy in OPA using PUT /v1/policies/{id}
func (c *HTTPClient) StorePolicy(ctx context.Context, policyID string, regoCode string) error {
	log := logging.FromContext(ctx)
	log.Debug("Storing policy in OPA", "policy_id", policyID)

	url := fmt.Sprintf(opaEndpointPattern, c.baseURL, policyID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, strings.NewReader(regoCode))
	if err != nil {
		return fmt.Errorf("%w: %v", ErrClientInternal, err)
	}
	req.Header.Set("Content-Type", "text/plain")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Error("OPA request failed", "operation", "store", "policy_id", policyID, "error", err)
		return fmt.Errorf("%w: %v", ErrOPAUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if err := handleStorePolicyResponse(resp); err != nil {
		log.Warn("OPA store policy returned error", "policy_id", policyID, "status", resp.StatusCode, "error", err)
		return err
	}

	log.Debug("Policy stored in OPA", "policy_id", policyID)
	return nil
}

func handleStorePolicyResponse(resp *http.Response) error {
	if resp.StatusCode == http.StatusOK {
		return nil
	}

	// Handle error responses
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusBadRequest {
		// Parse OPA error response
		var opaErr OPAError
		if err := json.Unmarshal(body, &opaErr); err == nil && len(opaErr.Errors) > 0 {
			return fmt.Errorf("%w: %s", ErrInvalidRego, opaErr.Errors[0])
		}
		return fmt.Errorf("%w: %s", ErrInvalidRego, string(body))
	}

	return fmt.Errorf("%w: status %d: %s", ErrOPAUnavailable, resp.StatusCode, string(body))
}

// GetPolicy retrieves a policy from OPA using GET /v1/policies/{id}
func (c *HTTPClient) GetPolicy(ctx context.Context, policyID string) (string, error) {
	log := logging.FromContext(ctx)
	log.Debug("Getting policy from OPA", "policy_id", policyID)

	url := fmt.Sprintf(opaEndpointPattern, c.baseURL, policyID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrClientInternal, err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Error("OPA request failed", "operation", "get", "policy_id", policyID, "error", err)
		return "", fmt.Errorf("%w: %v", ErrOPAUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		log.Warn("Policy not found in OPA", "policy_id", policyID)
		return "", ErrPolicyNotFound
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("%w: status %d: %s", ErrOPAUnavailable, resp.StatusCode, string(body))
	}

	// Parse the response to extract the raw Rego code
	var result struct {
		Result struct {
			Raw string `json:"raw"`
		} `json:"result"`
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("%w: failed to read response: %v", ErrClientInternal, err)
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("%w: failed to parse response: %v", ErrClientInternal, err)
	}

	return result.Result.Raw, nil
}

// DeletePolicy removes a policy from OPA using DELETE /v1/policies/{id}
func (c *HTTPClient) DeletePolicy(ctx context.Context, policyID string) error {
	log := logging.FromContext(ctx)
	log.Debug("Deleting policy from OPA", "policy_id", policyID)

	url := fmt.Sprintf(opaEndpointPattern, c.baseURL, policyID)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrClientInternal, err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Error("OPA request failed", "operation", "delete", "policy_id", policyID, "error", err)
		return fmt.Errorf("%w: %v", ErrOPAUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if err := handleDeletePolicyResponse(resp); err != nil {
		log.Warn("OPA delete policy returned error", "policy_id", policyID, "status", resp.StatusCode, "error", err)
		return err
	}

	log.Debug("Policy deleted from OPA", "policy_id", policyID)
	return nil
}

func handleDeletePolicyResponse(resp *http.Response) error {
	// DELETE is idempotent - both 200 and 404 are success
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNotFound {
		return nil
	}

	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("%w: status %d: %s", ErrOPAUnavailable, resp.StatusCode, string(body))
}

// EvaluatePolicy evaluates input against a policy using POST /v1/data/{packageName}/main
func (c *HTTPClient) EvaluatePolicy(ctx context.Context, packageName string, input map[string]any) (*EvaluationResult, error) {
	log := logging.FromContext(ctx)
	log.Debug("Evaluating policy in OPA", "package_name", packageName)

	// Convert package name dots to slashes for OPA data API path
	// e.g., "policies.my_policy" becomes "policies/my_policy"
	packagePath := strings.ReplaceAll(packageName, ".", "/")
	url := fmt.Sprintf("%s/v1/data/%s/main", c.baseURL, packagePath)

	// Construct the request body with input
	requestBody := map[string]any{
		"input": input,
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to marshal input: %v", ErrClientInternal, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrClientInternal, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Error("OPA request failed", "operation", "evaluate", "package_name", packageName, "error", err)
		return nil, fmt.Errorf("%w: %v", ErrOPAUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		log.Warn("Policy not found in OPA for evaluation", "package_name", packageName)
		return nil, ErrPolicyNotFound
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("%w: status %d: %s", ErrOPAUnavailable, resp.StatusCode, string(body))
	}

	// Parse the response
	var result struct {
		Result any `json:"result"`
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to read response: %v", ErrClientInternal, err)
	}

	// Fails when the response body is not valid JSON (e.g. malformed, truncated).
	// Does not fail when result is a non-object; that is caught by the type assertion below.
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("%w: failed to parse response: %v", ErrClientInternal, err)
	}

	// OPA returns {} when the policy is undefined; result.Result is nil
	if result.Result == nil {
		log.Debug("OPA policy evaluation returned undefined", "package_name", packageName)
		return &EvaluationResult{Result: nil, Defined: false}, nil
	}

	// Type-assert to map; if the policy returned a non-object, give a clear error
	resultMap, ok := result.Result.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%w: policy main rule must return an object (e.g. {\"rejected\": false}); got %T", ErrClientInternal, result.Result)
	}

	// Check if the result is defined (not undefined)
	// If the result map is present, the policy is defined
	evalResult := &EvaluationResult{
		Result:  resultMap,
		Defined: true,
	}

	log.Debug("OPA policy evaluation completed", "package_name", packageName, "defined", evalResult.Defined)
	return evalResult, nil
}
