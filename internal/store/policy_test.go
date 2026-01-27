package store_test

import (
	"context"

	"github.com/dcm-project/policy-manager/internal/store"
	"github.com/dcm-project/policy-manager/internal/store/model"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var _ = Describe("Policy Store", func() {
	var (
		db          *gorm.DB
		policyStore store.Policy
		ctx         context.Context
	)

	BeforeEach(func() {
		var err error
		db, err = gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(db.AutoMigrate(&model.Policy{})).To(Succeed())

		policyStore = store.NewPolicy(db)
		ctx = context.Background()
	})

	AfterEach(func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	})

	Describe("Create", func() {
		It("persists the policy", func() {
			p := newPolicy("create-test")
			created, err := policyStore.Create(ctx, p)

			Expect(err).NotTo(HaveOccurred())
			Expect(created.ID).To(Equal(p.ID))
			Expect(created.DisplayName).To(Equal("Create Test Policy"))
			Expect(created.PolicyType).To(Equal("GLOBAL"))
		})

		It("rejects duplicate IDs", func() {
			p1 := newPolicy("duplicate-id")
			_, err := policyStore.Create(ctx, p1)
			Expect(err).NotTo(HaveOccurred())

			p2 := newPolicy("duplicate-id")
			_, err = policyStore.Create(ctx, p2)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("Get", func() {
		It("retrieves by ID", func() {
			p := newPolicy("get-test")
			policyStore.Create(ctx, p)

			found, err := policyStore.Get(ctx, p.ID)

			Expect(err).NotTo(HaveOccurred())
			Expect(found.DisplayName).To(Equal("Get Test Policy"))
		})

		It("returns ErrPolicyNotFound for missing ID", func() {
			_, err := policyStore.Get(ctx, "non-existent-id")

			Expect(err).To(Equal(store.ErrPolicyNotFound))
		})
	})

	Describe("List", func() {
		It("returns all policies when filter is nil", func() {
			policyStore.Create(ctx, newPolicy("p1"))
			policyStore.Create(ctx, newPolicy("p2"))

			result, err := policyStore.List(ctx, nil)

			Expect(err).NotTo(HaveOccurred())
			Expect(result.Policies).To(HaveLen(2))
			Expect(result.NextPageToken).To(BeEmpty())
		})

		It("filters by policy type", func() {
			p1 := newPolicy("global-policy")
			p1.PolicyType = "GLOBAL"
			policyStore.Create(ctx, p1)

			p2 := newPolicy("user-policy")
			p2.PolicyType = "USER"
			policyStore.Create(ctx, p2)

			globalType := "GLOBAL"
			opts := &store.PolicyListOptions{
				Filter: &store.PolicyFilter{PolicyType: &globalType},
			}
			result, err := policyStore.List(ctx, opts)

			Expect(err).NotTo(HaveOccurred())
			Expect(result.Policies).To(HaveLen(1))
			Expect(result.Policies[0].ID).To(Equal("global-policy"))
		})

		It("filters by enabled status", func() {
			p1 := newPolicy("enabled-policy")
			p1.Enabled = true
			policyStore.Create(ctx, p1)

			p2 := newPolicy("disabled-policy")
			p2.Enabled = false
			policyStore.Create(ctx, p2)

			enabled := true
			opts := &store.PolicyListOptions{
				Filter: &store.PolicyFilter{Enabled: &enabled},
			}
			result, err := policyStore.List(ctx, opts)

			Expect(err).NotTo(HaveOccurred())
			Expect(result.Policies).To(HaveLen(1))
			Expect(result.Policies[0].ID).To(Equal("enabled-policy"))
		})

		It("filters by both policy type and enabled status", func() {
			p1 := newPolicy("global-enabled")
			p1.PolicyType = "GLOBAL"
			p1.Enabled = true
			policyStore.Create(ctx, p1)

			p2 := newPolicy("global-disabled")
			p2.PolicyType = "GLOBAL"
			p2.Enabled = false
			policyStore.Create(ctx, p2)

			p3 := newPolicy("user-enabled")
			p3.PolicyType = "USER"
			p3.Enabled = true
			policyStore.Create(ctx, p3)

			globalType := "GLOBAL"
			enabled := true
			opts := &store.PolicyListOptions{
				Filter: &store.PolicyFilter{
					PolicyType: &globalType,
					Enabled:    &enabled,
				},
			}
			result, err := policyStore.List(ctx, opts)

			Expect(err).NotTo(HaveOccurred())
			Expect(result.Policies).To(HaveLen(1))
			Expect(result.Policies[0].ID).To(Equal("global-enabled"))
		})

		It("orders policies by priority ascending by default", func() {
			p1 := newPolicy("low-priority")
			p1.Priority = 800
			policyStore.Create(ctx, p1)

			p2 := newPolicy("high-priority")
			p2.Priority = 100
			policyStore.Create(ctx, p2)

			p3 := newPolicy("medium-priority")
			p3.Priority = 400
			policyStore.Create(ctx, p3)

			result, err := policyStore.List(ctx, nil)

			Expect(err).NotTo(HaveOccurred())
			Expect(result.Policies).To(HaveLen(3))
			Expect(result.Policies[0].ID).To(Equal("high-priority"))
			Expect(result.Policies[1].ID).To(Equal("medium-priority"))
			Expect(result.Policies[2].ID).To(Equal("low-priority"))
		})

		It("applies custom ordering", func() {
			p1 := newPolicy("alpha")
			p1.DisplayName = "Zebra Policy"
			policyStore.Create(ctx, p1)

			p2 := newPolicy("beta")
			p2.DisplayName = "Alpha Policy"
			policyStore.Create(ctx, p2)

			opts := &store.PolicyListOptions{
				OrderBy: "display_name ASC",
			}
			result, err := policyStore.List(ctx, opts)

			Expect(err).NotTo(HaveOccurred())
			Expect(result.Policies).To(HaveLen(2))
			Expect(result.Policies[0].DisplayName).To(Equal("Alpha Policy"))
			Expect(result.Policies[1].DisplayName).To(Equal("Zebra Policy"))
		})

		It("applies page size for pagination", func() {
			for i := 1; i <= 5; i++ {
				p := newPolicy("policy-" + string(rune('0'+i)))
				p.Priority = int32(i * 100)
				policyStore.Create(ctx, p)
			}

			opts := &store.PolicyListOptions{
				PageSize: 2,
			}
			result, err := policyStore.List(ctx, opts)

			Expect(err).NotTo(HaveOccurred())
			Expect(result.Policies).To(HaveLen(2))
			Expect(result.Policies[0].Priority).To(Equal(int32(100)))
			Expect(result.Policies[1].Priority).To(Equal(int32(200)))
			Expect(result.NextPageToken).NotTo(BeEmpty())
		})

		It("uses page token for pagination across multiple pages", func() {
			for i := 1; i <= 5; i++ {
				p := newPolicy("policy-" + string(rune('0'+i)))
				p.Priority = int32(i * 100)
				policyStore.Create(ctx, p)
			}

			// First page
			opts := &store.PolicyListOptions{
				PageSize: 2,
			}
			result, err := policyStore.List(ctx, opts)

			Expect(err).NotTo(HaveOccurred())
			Expect(result.Policies).To(HaveLen(2))
			Expect(result.Policies[0].Priority).To(Equal(int32(100)))
			Expect(result.Policies[1].Priority).To(Equal(int32(200)))
			Expect(result.NextPageToken).NotTo(BeEmpty())

			// Second page using page token
			opts = &store.PolicyListOptions{
				PageSize:  2,
				PageToken: &result.NextPageToken,
			}
			result, err = policyStore.List(ctx, opts)

			Expect(err).NotTo(HaveOccurred())
			Expect(result.Policies).To(HaveLen(2))
			Expect(result.Policies[0].Priority).To(Equal(int32(300)))
			Expect(result.Policies[1].Priority).To(Equal(int32(400)))
			Expect(result.NextPageToken).NotTo(BeEmpty())

			// Third page (last page with 1 item)
			opts = &store.PolicyListOptions{
				PageSize:  2,
				PageToken: &result.NextPageToken,
			}
			result, err = policyStore.List(ctx, opts)

			Expect(err).NotTo(HaveOccurred())
			Expect(result.Policies).To(HaveLen(1))
			Expect(result.Policies[0].Priority).To(Equal(int32(500)))
			Expect(result.NextPageToken).To(BeEmpty())
		})

		It("uses default page size of 50 when not specified", func() {
			// Create 51 policies to test default page size
			for i := 1; i <= 51; i++ {
				p := newPolicy("policy-" + string(rune('0'+i)))
				p.Priority = int32(i * 10)
				policyStore.Create(ctx, p)
			}

			result, err := policyStore.List(ctx, &store.PolicyListOptions{})

			Expect(err).NotTo(HaveOccurred())
			Expect(result.Policies).To(HaveLen(50))
			Expect(result.NextPageToken).NotTo(BeEmpty())
		})
	})

	Describe("Delete", func() {
		It("removes the policy", func() {
			p := newPolicy("to-delete")
			policyStore.Create(ctx, p)

			err := policyStore.Delete(ctx, p.ID)

			Expect(err).NotTo(HaveOccurred())
		})

		It("returns ErrPolicyNotFound for missing ID", func() {
			err := policyStore.Delete(ctx, "non-existent-id")

			Expect(err).To(Equal(store.ErrPolicyNotFound))
		})
	})

	Describe("Update", func() {
		It("modifies existing policy", func() {
			p := newPolicy("to-update")
			policyStore.Create(ctx, p)

			p.DisplayName = "Updated Policy Name"
			p.Description = "Updated description"
			updated, err := policyStore.Update(ctx, p)

			Expect(err).NotTo(HaveOccurred())
			Expect(updated.DisplayName).To(Equal("Updated Policy Name"))
			Expect(updated.Description).To(Equal("Updated description"))
		})

		It("returns ErrPolicyNotFound for non-existing policy", func() {
			p := newPolicy("non-existing")
			_, err := policyStore.Update(ctx, p)

			Expect(err).To(Equal(store.ErrPolicyNotFound))
		})
	})
})

func newPolicy(id string) model.Policy {
	// Convert ID to a title-cased display name
	displayName := ""
	for i, c := range id {
		if c == '-' {
			displayName += " "
		} else if i == 0 || id[i-1] == '-' {
			displayName += string(c - 32) // Convert to uppercase
		} else {
			displayName += string(c)
		}
	}
	displayName += " Policy"

	return model.Policy{
		ID:          id,
		DisplayName: displayName,
		Description: "Test policy for " + id,
		PolicyType:  "GLOBAL",
		LabelSelector: map[string]string{
			"environment": "test",
		},
		Priority: 500,
		Enabled:  true,
	}
}
