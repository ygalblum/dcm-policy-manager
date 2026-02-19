package service

import (
	"context"
	"errors"

	"github.com/dcm-project/policy-manager/internal/opa"
	"github.com/dcm-project/policy-manager/internal/store"
	"github.com/dcm-project/policy-manager/internal/store/model"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Test suite is registered in other test files - don't register again

// Mock implementations
type mockPolicyStore struct {
	policies []model.Policy
	err      error
}

func (m *mockPolicyStore) Create(ctx context.Context, policy model.Policy) (*model.Policy, error) {
	return nil, errors.New("not implemented")
}

func (m *mockPolicyStore) Get(ctx context.Context, id string) (*model.Policy, error) {
	return nil, errors.New("not implemented")
}

func (m *mockPolicyStore) List(ctx context.Context, opts *store.PolicyListOptions) (*store.PolicyListResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &store.PolicyListResult{
		Policies: m.policies,
	}, nil
}

func (m *mockPolicyStore) Update(ctx context.Context, policy model.Policy) (*model.Policy, error) {
	return nil, errors.New("not implemented")
}

func (m *mockPolicyStore) Delete(ctx context.Context, id string) error {
	return errors.New("not implemented")
}

type mockOPAClient struct {
	evaluations map[string]*opa.EvaluationResult
	err         error
}

func (m *mockOPAClient) StorePolicy(ctx context.Context, policyID string, regoCode string) error {
	return errors.New("not implemented")
}

func (m *mockOPAClient) GetPolicy(ctx context.Context, policyID string) (string, error) {
	return "", errors.New("not implemented")
}

func (m *mockOPAClient) DeletePolicy(ctx context.Context, policyID string) error {
	return errors.New("not implemented")
}

func (m *mockOPAClient) EvaluatePolicy(ctx context.Context, packageName string, input map[string]interface{}) (*opa.EvaluationResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	if result, ok := m.evaluations[packageName]; ok {
		return result, nil
	}
	// Return undefined result by default
	return &opa.EvaluationResult{Defined: false}, nil
}

var _ = Describe("EvaluationService", func() {
	var (
		ctx         context.Context
		mockStore   *mockPolicyStore
		mockOPA     *mockOPAClient
		service     EvaluationService
		baseRequest *EvaluationRequest
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockStore = &mockPolicyStore{
			policies: []model.Policy{},
		}
		mockOPA = &mockOPAClient{
			evaluations: make(map[string]*opa.EvaluationResult),
		}
		service = NewEvaluationService(mockStore, mockOPA)

		baseRequest = &EvaluationRequest{
			ServiceInstance: map[string]interface{}{},
			RequestLabels:   map[string]string{},
		}
	})

	Describe("EvaluateRequest", func() {
		Context("when no policies exist", func() {
			It("returns approved with unchanged spec", func() {
				response, err := service.EvaluateRequest(ctx, baseRequest)

				Expect(err).NotTo(HaveOccurred())
				Expect(response.Status).To(Equal(EvaluationStatusApproved))
				Expect(response.EvaluatedServiceInstance).To(Equal(map[string]interface{}{}))
				Expect(response.SelectedProvider).To(Equal(""))
			})
		})

		Context("when policies don't match label selectors", func() {
			BeforeEach(func() {
				mockStore.policies = []model.Policy{
					{
						ID:            "policy-1",
						Enabled:       true,
						PolicyType:    "GLOBAL",
						Priority:      100,
						PackageName:   "policies.policy_1",
						LabelSelector: map[string]string{"env": "prod"},
					},
				}
			})

			It("returns approved with unchanged spec", func() {
				baseRequest.RequestLabels = map[string]string{"env": "dev"}

				response, err := service.EvaluateRequest(ctx, baseRequest)

				Expect(err).NotTo(HaveOccurred())
				Expect(response.Status).To(Equal(EvaluationStatusApproved))
			})
		})

		Context("when policy modifies the spec", func() {
			BeforeEach(func() {
				mockStore.policies = []model.Policy{
					{
						ID:          "policy-1",
						Enabled:     true,
						PolicyType:  "GLOBAL",
						Priority:    100,
						PackageName: "policies.policy_1",
					},
				}

				mockOPA.evaluations["policies.policy_1"] = &opa.EvaluationResult{
					Defined: true,
					Result: map[string]interface{}{
						"rejected": false,
						"output_spec": map[string]interface{}{
							"region": "us-east-1",
						},
						"selected_provider": "aws",
					},
				}
			})

			It("returns modified with updated spec", func() {
				response, err := service.EvaluateRequest(ctx, baseRequest)

				Expect(err).NotTo(HaveOccurred())
				Expect(response.Status).To(Equal(EvaluationStatusModified))
				Expect(response.EvaluatedServiceInstance).To(Equal(map[string]interface{}{
					"region": "us-east-1",
				}))
				Expect(response.SelectedProvider).To(Equal("aws"))
			})
		})

		Context("when policy rejects the request", func() {
			BeforeEach(func() {
				mockStore.policies = []model.Policy{
					{
						ID:          "policy-1",
						Enabled:     true,
						PolicyType:  "GLOBAL",
						Priority:    100,
						PackageName: "policies.policy_1",
					},
				}

				mockOPA.evaluations["policies.policy_1"] = &opa.EvaluationResult{
					Defined: true,
					Result: map[string]interface{}{
						"rejected":         true,
						"rejection_reason": "Security policy violation",
					},
				}
			})

			It("returns policy rejected error", func() {
				_, err := service.EvaluateRequest(ctx, baseRequest)

				Expect(err).To(HaveOccurred())
				serviceErr, ok := err.(*ServiceError)
				Expect(ok).To(BeTrue())
				Expect(serviceErr.Type).To(Equal(ErrorTypeRejected))
				Expect(serviceErr.Message).To(ContainSubstring("policy-1"))
				Expect(serviceErr.Detail).To(Equal("Security policy violation"))
			})
		})

		Context("when lower-priority policy violates constraint", func() {
			BeforeEach(func() {
				mockStore.policies = []model.Policy{
					{
						ID:          "policy-1",
						Enabled:     true,
						PolicyType:  "GLOBAL",
						Priority:    100,
						PackageName: "policies.policy_1",
					},
					{
						ID:          "policy-2",
						Enabled:     true,
						PolicyType:  "GLOBAL",
						Priority:    200,
						PackageName: "policies.policy_2",
					},
				}

				// First policy sets the region
				mockOPA.evaluations["policies.policy_1"] = &opa.EvaluationResult{
					Defined: true,
					Result: map[string]interface{}{
						"rejected": false,
						"output_spec": map[string]interface{}{
							"region": "us-east-1",
						},
					},
				}

				// Second policy tries to change the region
				mockOPA.evaluations["policies.policy_2"] = &opa.EvaluationResult{
					Defined: true,
					Result: map[string]interface{}{
						"rejected": false,
						"output_spec": map[string]interface{}{
							"region": "us-west-2",
						},
					},
				}
			})

			It("returns policy conflict error", func() {
				_, err := service.EvaluateRequest(ctx, baseRequest)

				Expect(err).To(HaveOccurred())
				serviceErr, ok := err.(*ServiceError)
				Expect(ok).To(BeTrue())
				Expect(serviceErr.Type).To(Equal(ErrorTypePolicyConflict))
				Expect(serviceErr.Message).To(ContainSubstring("policy-2"))
				Expect(serviceErr.Message).To(ContainSubstring("policy-1"))
			})
		})

		Context("when policies evaluate sequentially without conflicts", func() {
			BeforeEach(func() {
				mockStore.policies = []model.Policy{
					{
						ID:          "policy-1",
						Enabled:     true,
						PolicyType:  "GLOBAL",
						Priority:    100,
						PackageName: "policies.policy_1",
					},
					{
						ID:          "policy-2",
						Enabled:     true,
						PolicyType:  "USER",
						Priority:    100,
						PackageName: "policies.policy_2",
					},
				}

				// First policy adds region
				mockOPA.evaluations["policies.policy_1"] = &opa.EvaluationResult{
					Defined: true,
					Result: map[string]interface{}{
						"rejected": false,
						"output_spec": map[string]interface{}{
							"region": "us-east-1",
						},
					},
				}

				// Second policy adds instance_type (no conflict)
				mockOPA.evaluations["policies.policy_2"] = &opa.EvaluationResult{
					Defined: true,
					Result: map[string]interface{}{
						"rejected": false,
						"output_spec": map[string]interface{}{
							"region":        "us-east-1",
							"instance_type": "t3.medium",
						},
					},
				}
			})

			It("applies both policies successfully", func() {
				response, err := service.EvaluateRequest(ctx, baseRequest)

				Expect(err).NotTo(HaveOccurred())
				Expect(response.Status).To(Equal(EvaluationStatusModified))
				Expect(response.EvaluatedServiceInstance).To(Equal(map[string]interface{}{
					"region":        "us-east-1",
					"instance_type": "t3.medium",
				}))
			})
		})

		Context("when policy evaluation fails", func() {
			BeforeEach(func() {
				mockStore.policies = []model.Policy{
					{
						ID:          "policy-1",
						Enabled:     true,
						PolicyType:  "GLOBAL",
						Priority:    100,
						PackageName: "policies.policy_1",
					},
				}

				mockOPA.err = errors.New("OPA unavailable")
			})

			It("returns internal error", func() {
				_, err := service.EvaluateRequest(ctx, baseRequest)

				Expect(err).To(HaveOccurred())
				serviceErr, ok := err.(*ServiceError)
				Expect(ok).To(BeTrue())
				Expect(serviceErr.Type).To(Equal(ErrorTypeInternal))
			})
		})

		Context("when label selector matches", func() {
			BeforeEach(func() {
				mockStore.policies = []model.Policy{
					{
						ID:            "policy-1",
						Enabled:       true,
						PolicyType:    "GLOBAL",
						Priority:      100,
						PackageName:   "policies.policy_1",
						LabelSelector: map[string]string{"env": "prod", "team": "backend"},
					},
				}

				mockOPA.evaluations["policies.policy_1"] = &opa.EvaluationResult{
					Defined: true,
					Result: map[string]interface{}{
						"rejected": false,
						"output_spec": map[string]interface{}{
							"region": "us-east-1",
						},
					},
				}
			})

			It("applies policy when all labels match", func() {
				baseRequest.RequestLabels = map[string]string{
					"env":  "prod",
					"team": "backend",
					"app":  "web", // Extra label is OK
				}

				response, err := service.EvaluateRequest(ctx, baseRequest)

				Expect(err).NotTo(HaveOccurred())
				Expect(response.Status).To(Equal(EvaluationStatusModified))
			})

			It("skips policy when labels don't match", func() {
				baseRequest.RequestLabels = map[string]string{
					"env": "prod",
					// Missing "team" label
				}

				response, err := service.EvaluateRequest(ctx, baseRequest)

				Expect(err).NotTo(HaveOccurred())
				Expect(response.Status).To(Equal(EvaluationStatusApproved))
			})
		})
	})
})
