package opa_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/dcm-project/policy-manager/internal/opa"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("OPA Client", func() {
	var (
		ctx context.Context
	)

	BeforeEach(func() {
		ctx = context.Background()
	})

	Describe("StorePolicy", func() {
		It("successfully stores a valid policy", func() {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.Method).To(Equal(http.MethodPut))
				Expect(r.URL.Path).To(Equal("/v1/policies/test-policy"))
				Expect(r.Header.Get("Content-Type")).To(Equal("text/plain"))
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			client := opa.NewClient(server.URL, 5*time.Second)
			err := client.StorePolicy(ctx, "test-policy", "package test\n\ndefault allow = false")

			Expect(err).NotTo(HaveOccurred())
		})

		It("returns ErrInvalidRego for invalid Rego code", func() {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				opaErr := opa.OPAError{
					Code:    "invalid_parameter",
					Message: "error(s) occurred while compiling module(s)",
					Errors:  []string{"rego_parse_error: invalid syntax"},
				}
				json.NewEncoder(w).Encode(opaErr)
			}))
			defer server.Close()

			client := opa.NewClient(server.URL, 5*time.Second)
			err := client.StorePolicy(ctx, "test-policy", "invalid rego")

			Expect(err).To(MatchError(ContainSubstring("invalid Rego code")))
		})

		It("returns ErrOPAUnavailable when OPA is unreachable", func() {
			client := opa.NewClient("http://localhost:1", 100*time.Millisecond)
			err := client.StorePolicy(ctx, "test-policy", "package test")

			Expect(err).To(MatchError(ContainSubstring("OPA service unavailable")))
		})

		It("handles base URL with trailing slash", func() {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.URL.Path).To(Equal("/v1/policies/test-policy"))
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			client := opa.NewClient(server.URL+"/", 5*time.Second)
			err := client.StorePolicy(ctx, "test-policy", "package test")

			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("GetPolicy", func() {
		It("successfully retrieves a policy", func() {
			expectedRego := "package test\n\ndefault allow = false"
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.Method).To(Equal(http.MethodGet))
				Expect(r.URL.Path).To(Equal("/v1/policies/test-policy"))
				w.WriteHeader(http.StatusOK)
				response := map[string]any{
					"result": map[string]any{
						"raw": expectedRego,
					},
				}
				json.NewEncoder(w).Encode(response)
			}))
			defer server.Close()

			client := opa.NewClient(server.URL, 5*time.Second)
			rego, err := client.GetPolicy(ctx, "test-policy")

			Expect(err).NotTo(HaveOccurred())
			Expect(rego).To(Equal(expectedRego))
		})

		It("returns ErrPolicyNotFound when policy doesn't exist", func() {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				json.NewEncoder(w).Encode(map[string]string{
					"code": "not_found",
				})
			}))
			defer server.Close()

			client := opa.NewClient(server.URL, 5*time.Second)
			_, err := client.GetPolicy(ctx, "nonexistent")

			Expect(err).To(MatchError(opa.ErrPolicyNotFound))
		})

		It("returns ErrOPAUnavailable when OPA is unreachable", func() {
			client := opa.NewClient("http://localhost:1", 100*time.Millisecond)
			_, err := client.GetPolicy(ctx, "test-policy")

			Expect(err).To(MatchError(ContainSubstring("OPA service unavailable")))
		})

		It("returns ErrOPAUnavailable for non-200/404 status codes", func() {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			}))
			defer server.Close()

			client := opa.NewClient(server.URL, 5*time.Second)
			_, err := client.GetPolicy(ctx, "test-policy")

			Expect(err).To(MatchError(ContainSubstring("OPA service unavailable")))
		})

		It("returns ErrClientInternal when failing to parse response", func() {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("not a valid JSON"))
			}))
			defer server.Close()

			client := opa.NewClient(server.URL, 5*time.Second)
			_, err := client.GetPolicy(ctx, "test-policy")

			Expect(err).To(MatchError(ContainSubstring("client internal error")))
			Expect(err).To(MatchError(ContainSubstring("failed to parse response")))
		})
	})

	Describe("DeletePolicy", func() {
		It("successfully deletes a policy", func() {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.Method).To(Equal(http.MethodDelete))
				Expect(r.URL.Path).To(Equal("/v1/policies/test-policy"))
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			client := opa.NewClient(server.URL, 5*time.Second)
			err := client.DeletePolicy(ctx, "test-policy")

			Expect(err).NotTo(HaveOccurred())
		})

		It("treats 404 as success (idempotent)", func() {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			}))
			defer server.Close()

			client := opa.NewClient(server.URL, 5*time.Second)
			err := client.DeletePolicy(ctx, "nonexistent")

			Expect(err).NotTo(HaveOccurred())
		})

		It("returns ErrOPAUnavailable when OPA is unreachable", func() {
			client := opa.NewClient("http://localhost:1", 100*time.Millisecond)
			err := client.DeletePolicy(ctx, "test-policy")

			Expect(err).To(MatchError(ContainSubstring("OPA service unavailable")))
		})

		It("returns ErrOPAUnavailable for non-200/404 status codes", func() {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			}))
			defer server.Close()

			client := opa.NewClient(server.URL, 5*time.Second)
			err := client.DeletePolicy(ctx, "test-policy")

			Expect(err).To(MatchError(ContainSubstring("OPA service unavailable")))
		})
	})

	Describe("EvaluatePolicy", func() {
		It("successfully evaluates a policy with defined result", func() {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.Method).To(Equal(http.MethodPost))
				Expect(r.URL.Path).To(Equal("/v1/data/policies/test_policy/main"))
				Expect(r.Header.Get("Content-Type")).To(Equal("application/json"))

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				response := map[string]any{
					"result": map[string]any{
						"rejected": false,
						"output_spec": map[string]any{
							"provider": "aws",
							"region":   "us-east-1",
						},
						"selected_provider": "aws",
					},
				}
				json.NewEncoder(w).Encode(response)
			}))
			defer server.Close()

			client := opa.NewClient(server.URL, 5*time.Second)
			result, err := client.EvaluatePolicy(ctx, "policies.test_policy", map[string]interface{}{
				"spec": map[string]interface{}{
					"region": "us-east-1",
				},
				"provider": "aws",
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(result.Defined).To(BeTrue())
			Expect(result.Result).NotTo(BeNil())

			decision := opa.ParsePolicyDecision(result.Result)
			Expect(decision.Rejected).To(BeFalse())
			Expect(decision.SelectedProvider).To(Equal("aws"))
		})

		It("handles undefined result", func() {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.Method).To(Equal(http.MethodPost))
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]any{})
			}))
			defer server.Close()

			client := opa.NewClient(server.URL, 5*time.Second)
			result, err := client.EvaluatePolicy(ctx, "policies.test_policy", map[string]interface{}{})

			Expect(err).NotTo(HaveOccurred())
			Expect(result.Defined).To(BeFalse())
		})

		It("returns ErrPolicyNotFound when policy doesn't exist", func() {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			}))
			defer server.Close()

			client := opa.NewClient(server.URL, 5*time.Second)
			_, err := client.EvaluatePolicy(ctx, "nonexistent", map[string]interface{}{})

			Expect(err).To(MatchError(opa.ErrPolicyNotFound))
		})

		It("returns ErrOPAUnavailable when OPA is unreachable", func() {
			client := opa.NewClient("http://localhost:1", 100*time.Millisecond)
			_, err := client.EvaluatePolicy(ctx, "test_policy", map[string]interface{}{})

			Expect(err).To(MatchError(ContainSubstring("OPA service unavailable")))
		})

		It("returns ErrOPAUnavailable for non-200/404 status codes", func() {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			}))
			defer server.Close()

			client := opa.NewClient(server.URL, 5*time.Second)
			_, err := client.EvaluatePolicy(ctx, "test_policy", map[string]interface{}{})

			Expect(err).To(MatchError(ContainSubstring("OPA service unavailable")))
		})
	})
})
