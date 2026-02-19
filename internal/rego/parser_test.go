package rego

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestRego(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Rego Parser Suite")
}

var _ = Describe("ExtractPackageName", func() {
	Context("Valid package declarations", func() {
		It("should extract simple package name", func() {
			regoCode := `package test

allow {
    true
}`
			pkgName, err := ExtractPackageName(regoCode)
			Expect(err).ToNot(HaveOccurred())
			Expect(pkgName).To(Equal("test"))
		})

		It("should extract namespaced package name", func() {
			regoCode := `package policies.my_policy

allow {
    true
}`
			pkgName, err := ExtractPackageName(regoCode)
			Expect(err).ToNot(HaveOccurred())
			Expect(pkgName).To(Equal("policies.my_policy"))
		})

		It("should extract multi-level namespaced package", func() {
			regoCode := `package a.b.c

allow {
    true
}`
			pkgName, err := ExtractPackageName(regoCode)
			Expect(err).ToNot(HaveOccurred())
			Expect(pkgName).To(Equal("a.b.c"))
		})

		It("should handle package with trailing comment", func() {
			regoCode := `package test # this is a comment

allow {
    true
}`
			pkgName, err := ExtractPackageName(regoCode)
			Expect(err).ToNot(HaveOccurred())
			Expect(pkgName).To(Equal("test"))
		})

		It("should handle package with extra whitespace", func() {
			regoCode := `package   test

allow {
    true
}`
			pkgName, err := ExtractPackageName(regoCode)
			Expect(err).ToNot(HaveOccurred())
			Expect(pkgName).To(Equal("test"))
		})

		It("should handle package declaration after empty lines", func() {
			regoCode := `

package test

allow {
    true
}`
			pkgName, err := ExtractPackageName(regoCode)
			Expect(err).ToNot(HaveOccurred())
			Expect(pkgName).To(Equal("test"))
		})

		It("should handle package declaration after comment lines", func() {
			regoCode := `# This is a header comment
# Another comment line

package test

allow {
    true
}`
			pkgName, err := ExtractPackageName(regoCode)
			Expect(err).ToNot(HaveOccurred())
			Expect(pkgName).To(Equal("test"))
		})
	})

	Context("Invalid package declarations", func() {
		It("should return error when no package declaration found", func() {
			regoCode := `allow {
    true
}`
			_, err := ExtractPackageName(regoCode)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("no package declaration found"))
		})

		It("should return error when package declaration is empty", func() {
			regoCode := `package

allow {
    true
}`
			_, err := ExtractPackageName(regoCode)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("empty package declaration"))
		})

		It("should return error when package declaration is only whitespace", func() {
			regoCode := `package

allow {
    true
}`
			_, err := ExtractPackageName(regoCode)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("empty package declaration"))
		})

		It("should return error when only comments exist", func() {
			regoCode := `# Just comments
# No package declaration`
			_, err := ExtractPackageName(regoCode)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("no package declaration found"))
		})

		It("should return error when empty Rego code", func() {
			regoCode := ``
			_, err := ExtractPackageName(regoCode)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("no package declaration found"))
		})
	})
})
