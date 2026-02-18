package opa

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
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
	url := fmt.Sprintf(opaEndpointPattern, c.baseURL, policyID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, strings.NewReader(regoCode))
	if err != nil {
		return fmt.Errorf("%w: %v", ErrClientInternal, err)
	}
	req.Header.Set("Content-Type", "text/plain")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrOPAUnavailable, err)
	}
	defer resp.Body.Close()

	return handleStorePolicyResponse(resp)
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
	url := fmt.Sprintf(opaEndpointPattern, c.baseURL, policyID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrClientInternal, err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrOPAUnavailable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
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
	url := fmt.Sprintf(opaEndpointPattern, c.baseURL, policyID)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrClientInternal, err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrOPAUnavailable, err)
	}
	defer resp.Body.Close()

	return handleDeletePolicyResponse(resp)
}

func handleDeletePolicyResponse(resp *http.Response) error {
	// DELETE is idempotent - both 200 and 404 are success
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNotFound {
		return nil
	}

	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("%w: status %d: %s", ErrOPAUnavailable, resp.StatusCode, string(body))
}
