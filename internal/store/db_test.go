package store_test

import (
	"github.com/dcm-project/policy-manager/internal/config"
	"github.com/dcm-project/policy-manager/internal/store"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("InitDB", func() {
	It("initializes SQLite database", func() {
		cfg := &config.Config{
			Database: &config.DBConfig{
				Type: "sqlite",
				Name: ":memory:",
			},
		}

		db, err := store.InitDB(cfg)

		Expect(err).NotTo(HaveOccurred())
		Expect(db).NotTo(BeNil())

		sqlDB, _ := db.DB()
		sqlDB.Close()
	})
})
