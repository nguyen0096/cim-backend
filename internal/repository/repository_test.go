package repository

import (
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"cim-backend/database"
	"cim-backend/internal/config"
)

var db *gorm.DB

var _ = BeforeSuite(func(ctx SpecContext) {

	By("Setup test database", func() {
		// Load test configuration
		cfg := config.Load()

		// Initialize test database
		var err error
		db, err = database.Initialize(cfg.Database)
		if err != nil {
			Fail(fmt.Sprintf("Failed to initialize test database: %v", err))
		}

		// set logger to error level to shorten test output
		db.Logger.LogMode(logger.Error)
	})
})

func TestRepository(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Repository Test Suite")
}
