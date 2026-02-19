//go:build e2e

package e2e_test

import (
	"context"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/dcm-project/policy-manager/api/v1alpha1"
	engineapi "github.com/dcm-project/policy-manager/api/v1alpha1/engine"
	"github.com/dcm-project/policy-manager/pkg/client"
	"github.com/dcm-project/policy-manager/pkg/engineclient"
)

var _ = Describe("Engine API - Policy Evaluation", func() {
	var (
		engineClient *engineclient.ClientWithResponses
		policyClient *client.ClientWithResponses
		ctx          context.Context
	)

	BeforeEach(func() {
		engineURL := getEnvOrDefault("ENGINE_API_URL", "http://localhost:8081/api/v1alpha1")
		policyURL := getEnvOrDefault("API_URL", "http://localhost:8080/api/v1alpha1")

		var err error
		engineClient, err = engineclient.NewClientWithResponses(engineURL)
		Expect(err).NotTo(HaveOccurred())

		policyClient, err = client.NewClientWithResponses(policyURL)
		Expect(err).NotTo(HaveOccurred())

		ctx = context.Background()
	})

	Describe("POST /policies:evaluateRequest", func() {
		Context("when no policies exist", func() {
			It("should return APPROVED with unchanged spec", func() {
				request := engineapi.EvaluateRequest{
					ServiceInstance: engineapi.ServiceInstance{
						Spec: map[string]any{
							"region": "us-east-1",
						},
					},
				}

				resp, err := engineClient.EvaluateRequestWithResponse(ctx, request)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode()).To(Equal(http.StatusOK))
				Expect(resp.JSON200).NotTo(BeNil())
				Expect(resp.JSON200.Status).To(Equal(engineapi.APPROVED))
				Expect(resp.JSON200.EvaluatedServiceInstance.Spec).To(Equal(request.ServiceInstance.Spec))
				Expect(resp.JSON200.SelectedProvider).To(Equal(""))
			})
		})

		Context("when policy modifies the spec", func() {
			var policyID string

			BeforeEach(func() {
				// Create a policy that adds a region field
				regoCode := `package policies.test_modify

main := {
	"rejected": false,
	"output_spec": {
		"region": "us-west-2",
		"instance_type": "t3.medium"
	},
	"selected_provider": "aws"
}`
				policyID = "test-modify-policy"
				displayName := "Test Modify Policy"
				policyType := v1alpha1.GLOBAL
				enabled := true
				priority := int32(100)

				createResp, err := policyClient.CreatePolicyWithResponse(ctx, &v1alpha1.CreatePolicyParams{
					Id: &policyID,
				}, v1alpha1.Policy{
					DisplayName: &displayName,
					PolicyType:  &policyType,
					RegoCode:    &regoCode,
					Enabled:     &enabled,
					Priority:    &priority,
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(createResp.StatusCode()).To(Equal(http.StatusCreated))
			})

			AfterEach(func() {
				policyClient.DeletePolicyWithResponse(ctx, policyID)
			})

			It("should return MODIFIED with updated spec", func() {
				request := engineapi.EvaluateRequest{
					ServiceInstance: engineapi.ServiceInstance{
						Spec: map[string]any{},
					},
				}

				resp, err := engineClient.EvaluateRequestWithResponse(ctx, request)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode()).To(Equal(http.StatusOK))
				Expect(resp.JSON200).NotTo(BeNil())
				Expect(resp.JSON200.Status).To(Equal(engineapi.MODIFIED))
				Expect(resp.JSON200.EvaluatedServiceInstance.Spec["region"]).To(Equal("us-west-2"))
				Expect(resp.JSON200.EvaluatedServiceInstance.Spec["instance_type"]).To(Equal("t3.medium"))
				Expect(resp.JSON200.SelectedProvider).To(Equal("aws"))
			})
		})

		Context("when policy rejects the request", func() {
			var policyID string

			BeforeEach(func() {
				// Create a policy that rejects requests
				regoCode := `package policies.test_reject

main := {
	"rejected": true,
	"rejection_reason": "Test security policy violation"
}`
				policyID = "test-reject-policy"
				displayName := "Test Reject Policy"
				policyType := v1alpha1.GLOBAL
				enabled := true
				priority := int32(100)

				createResp, err := policyClient.CreatePolicyWithResponse(ctx, &v1alpha1.CreatePolicyParams{
					Id: &policyID,
				}, v1alpha1.Policy{
					DisplayName: &displayName,
					PolicyType:  &policyType,
					RegoCode:    &regoCode,
					Enabled:     &enabled,
					Priority:    &priority,
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(createResp.StatusCode()).To(Equal(http.StatusCreated))
			})

			AfterEach(func() {
				policyClient.DeletePolicyWithResponse(ctx, policyID)
			})

			It("should return 406 Not Acceptable", func() {
				request := engineapi.EvaluateRequest{
					ServiceInstance: engineapi.ServiceInstance{
						Spec: map[string]any{},
					},
				}

				resp, err := engineClient.EvaluateRequestWithResponse(ctx, request)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode()).To(Equal(http.StatusNotAcceptable))
				Expect(resp.JSON406).NotTo(BeNil())
				Expect(resp.JSON406.Detail).NotTo(BeNil())
				Expect(*resp.JSON406.Detail).To(ContainSubstring("Test security policy violation"))
			})
		})

		Context("when lower-priority policy violates constraint", func() {
			var policy1ID, policy2ID string

			BeforeEach(func() {
				// Create first policy that sets region
				regoCode1 := `package policies.test_constraint1

main := {
	"rejected": false,
	"output_spec": {
		"region": "us-east-1"
	},
	"selected_provider": input.provider
}`
				policy1ID = "test-constraint-policy-1"
				displayName1 := "Test Constraint Policy 1"
				policyType1 := v1alpha1.GLOBAL
				enabled1 := true
				priority1 := int32(100)

				createResp1, err := policyClient.CreatePolicyWithResponse(ctx, &v1alpha1.CreatePolicyParams{
					Id: &policy1ID,
				}, v1alpha1.Policy{
					DisplayName: &displayName1,
					PolicyType:  &policyType1,
					RegoCode:    &regoCode1,
					Enabled:     &enabled1,
					Priority:    &priority1,
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(createResp1.StatusCode()).To(Equal(http.StatusCreated))

				// Create second policy that tries to change region
				regoCode2 := `package policies.test_constraint2

main := {
	"rejected": false,
	"output_spec": {
		"region": "us-west-2"
	},
	"selected_provider": input.provider
}`
				policy2ID = "test-constraint-policy-2"
				displayName2 := "Test Constraint Policy 2"
				policyType2 := v1alpha1.GLOBAL
				enabled2 := true
				priority2 := int32(200)

				createResp2, err := policyClient.CreatePolicyWithResponse(ctx, &v1alpha1.CreatePolicyParams{
					Id: &policy2ID,
				}, v1alpha1.Policy{
					DisplayName: &displayName2,
					PolicyType:  &policyType2,
					RegoCode:    &regoCode2,
					Enabled:     &enabled2,
					Priority:    &priority2,
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(createResp2.StatusCode()).To(Equal(http.StatusCreated))
			})

			AfterEach(func() {
				policyClient.DeletePolicyWithResponse(ctx, policy1ID)
				policyClient.DeletePolicyWithResponse(ctx, policy2ID)
			})

			It("should return 409 Conflict", func() {
				request := engineapi.EvaluateRequest{
					ServiceInstance: engineapi.ServiceInstance{
						Spec: map[string]any{},
					},
				}

				resp, err := engineClient.EvaluateRequestWithResponse(ctx, request)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode()).To(Equal(http.StatusConflict))
				Expect(resp.JSON409).NotTo(BeNil())
			})
		})

		Context("when label selector matches", func() {
			var policyID string

			BeforeEach(func() {
				// Create a policy with label selector
				regoCode := `package policies.test_labels

main := {
	"rejected": false,
	"output_spec": {
		"env": "production"
	},
	"selected_provider": "aws"
}`
				policyID = "test-label-policy"
				displayName := "Test Label Policy"
				policyType := v1alpha1.GLOBAL
				enabled := true
				priority := int32(100)
				labelSelector := map[string]string{
					"env":  "prod",
					"team": "backend",
				}

				createResp, err := policyClient.CreatePolicyWithResponse(ctx, &v1alpha1.CreatePolicyParams{
					Id: &policyID,
				}, v1alpha1.Policy{
					DisplayName:   &displayName,
					PolicyType:    &policyType,
					RegoCode:      &regoCode,
					Enabled:       &enabled,
					Priority:      &priority,
					LabelSelector: &labelSelector,
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(createResp.StatusCode()).To(Equal(http.StatusCreated))
			})

			AfterEach(func() {
				policyClient.DeletePolicyWithResponse(ctx, policyID)
			})

			It("should apply policy when labels match", func() {
				request := engineapi.EvaluateRequest{
					ServiceInstance: engineapi.ServiceInstance{
						Spec: map[string]any{
							"metadata": map[string]any{
								"labels": map[string]any{
									"env":  "prod",
									"team": "backend",
								},
							},
						},
					},
				}

				resp, err := engineClient.EvaluateRequestWithResponse(ctx, request)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode()).To(Equal(http.StatusOK))
				Expect(resp.JSON200.Status).To(Equal(engineapi.MODIFIED))
				Expect(resp.JSON200.EvaluatedServiceInstance.Spec["env"]).To(Equal("production"))
				Expect(resp.JSON200.SelectedProvider).To(Equal("aws"))
			})

			It("should skip policy when labels don't match", func() {
				request := engineapi.EvaluateRequest{
					ServiceInstance: engineapi.ServiceInstance{
						Spec: map[string]any{
							"metadata": map[string]any{
								"labels": map[string]any{
									"env": "dev",
								},
							},
						},
					},
				}

				resp, err := engineClient.EvaluateRequestWithResponse(ctx, request)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode()).To(Equal(http.StatusOK))
				Expect(resp.JSON200.Status).To(Equal(engineapi.APPROVED))
				Expect(resp.JSON200.SelectedProvider).To(Equal(""))
			})
		})
	})
})
