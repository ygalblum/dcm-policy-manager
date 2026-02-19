package service

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Test suite is registered in other test files - don't register again

var _ = Describe("ConstraintContext", func() {
	var ctx *ConstraintContext

	BeforeEach(func() {
		ctx = NewConstraintContext()
	})

	Describe("MarkImmutable and IsImmutable", func() {
		It("marks a field as immutable", func() {
			ctx.MarkImmutable("spec.provider", "policy-1")

			Expect(ctx.IsImmutable("spec.provider")).To(BeTrue())
			Expect(ctx.GetSetBy("spec.provider")).To(Equal("policy-1"))
		})

		It("returns false for unmarked fields", func() {
			Expect(ctx.IsImmutable("spec.region")).To(BeFalse())
			Expect(ctx.GetSetBy("spec.region")).To(Equal(""))
		})
	})

	Describe("CheckViolations", func() {
		It("detects no violations when fields are unchanged", func() {
			original := map[string]interface{}{
				"provider": "aws",
				"region":   "us-east-1",
			}
			modified := map[string]interface{}{
				"provider": "aws",
				"region":   "us-east-1",
			}

			ctx.MarkImmutable("provider", "policy-1")
			violations := ctx.CheckViolations(original, modified)

			Expect(violations).To(BeEmpty())
		})

		It("detects violations when immutable field is changed", func() {
			original := map[string]interface{}{
				"provider": "aws",
			}
			modified := map[string]interface{}{
				"provider": "gcp",
			}

			ctx.MarkImmutable("provider", "policy-1")
			violations := ctx.CheckViolations(original, modified)

			Expect(violations).To(ConsistOf("provider"))
		})

		It("allows changes to non-immutable fields", func() {
			original := map[string]interface{}{
				"provider": "aws",
				"region":   "us-east-1",
			}
			modified := map[string]interface{}{
				"provider": "aws",
				"region":   "us-west-2",
			}

			ctx.MarkImmutable("provider", "policy-1")
			violations := ctx.CheckViolations(original, modified)

			Expect(violations).To(BeEmpty())
		})

		It("detects violations in nested fields", func() {
			original := map[string]interface{}{
				"compute": map[string]interface{}{
					"instance_type": "t3.medium",
				},
			}
			modified := map[string]interface{}{
				"compute": map[string]interface{}{
					"instance_type": "t3.large",
				},
			}

			ctx.MarkImmutable("compute.instance_type", "policy-1")
			violations := ctx.CheckViolations(original, modified)

			Expect(violations).To(ConsistOf("compute.instance_type"))
		})

		It("detects multiple violations", func() {
			original := map[string]interface{}{
				"provider": "aws",
				"region":   "us-east-1",
			}
			modified := map[string]interface{}{
				"provider": "gcp",
				"region":   "us-west-2",
			}

			ctx.MarkImmutable("provider", "policy-1")
			ctx.MarkImmutable("region", "policy-1")
			violations := ctx.CheckViolations(original, modified)

			Expect(violations).To(ConsistOf("provider", "region"))
		})
	})

	Describe("MarkChangedFields", func() {
		It("marks changed top-level fields as immutable", func() {
			original := map[string]interface{}{
				"provider": "aws",
				"region":   "us-east-1",
			}
			modified := map[string]interface{}{
				"provider": "aws",
				"region":   "us-west-2",
			}

			ctx.MarkChangedFields(original, modified, "policy-1")

			Expect(ctx.IsImmutable("provider")).To(BeFalse()) // Unchanged
			Expect(ctx.IsImmutable("region")).To(BeTrue())    // Changed
			Expect(ctx.GetSetBy("region")).To(Equal("policy-1"))
		})

		It("marks changed nested fields as immutable", func() {
			original := map[string]interface{}{
				"compute": map[string]interface{}{
					"instance_type": "t3.medium",
					"disk_size":     100,
				},
			}
			modified := map[string]interface{}{
				"compute": map[string]interface{}{
					"instance_type": "t3.large",
					"disk_size":     100,
				},
			}

			ctx.MarkChangedFields(original, modified, "policy-1")

			Expect(ctx.IsImmutable("compute.instance_type")).To(BeTrue())
			Expect(ctx.IsImmutable("compute.disk_size")).To(BeFalse())
		})

		It("marks newly added fields as immutable", func() {
			original := map[string]interface{}{
				"provider": "aws",
			}
			modified := map[string]interface{}{
				"provider": "aws",
				"region":   "us-east-1",
			}

			ctx.MarkChangedFields(original, modified, "policy-1")

			Expect(ctx.IsImmutable("region")).To(BeTrue())
		})

		It("marks entire nested object when type changes", func() {
			original := map[string]interface{}{
				"compute": "simple",
			}
			modified := map[string]interface{}{
				"compute": map[string]interface{}{
					"instance_type": "t3.medium",
				},
			}

			ctx.MarkChangedFields(original, modified, "policy-1")

			Expect(ctx.IsImmutable("compute")).To(BeTrue())
		})
	})

	Describe("FormatViolationError", func() {
		It("formats a single violation", func() {
			ctx.MarkImmutable("provider", "policy-1")
			violations := []string{"provider"}

			msg := FormatViolationError(violations, ctx)
			Expect(msg).To(Equal("Constraint violations: provider (set by policy-1)"))
		})

		It("formats multiple violations", func() {
			ctx.MarkImmutable("provider", "policy-1")
			ctx.MarkImmutable("region", "policy-2")
			violations := []string{"provider", "region"}

			msg := FormatViolationError(violations, ctx)
			Expect(msg).To(ContainSubstring("provider (set by policy-1)"))
			Expect(msg).To(ContainSubstring("region (set by policy-2)"))
		})

		It("returns empty string for no violations", func() {
			violations := []string{}

			msg := FormatViolationError(violations, ctx)
			Expect(msg).To(Equal(""))
		})
	})
})
