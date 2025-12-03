package testutil

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/labstack/echo/v4"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/gorm"

	"cim-backend/database"
	"cim-backend/internal/auth/authmocks"
	"cim-backend/internal/config"
	"cim-backend/internal/server"
	"cim-backend/pkg"
	"cim-backend/pkg/log"
)

const (
	testDBUser     = "test_user"
	testDBPassword = "test_password"
	testDBName     = "test_db"
	testDBPort     = "5432"

	testServerPortBase = 16081

	suiteSetupStepTimeout    = 5 * time.Second
	suiteProvisionTimeout    = 30 * time.Second
	suiteTeardownStepTimeout = 5 * time.Second
	suiteDeprovisionTimeout  = 10 * time.Second
)

// TestBox holds all test components like infrastructure (database, server, etc.),
// mocks, and default context.
type TestBox struct {
	Config      config.Config
	DBContainer testcontainers.Container
	DB          *gorm.DB
	Server      *echo.Echo
	BaseURL     string
	Cleanup     func()

	// Default context
	DefaultContext context.Context

	// Mocks
	AuthMock *authmocks.FirebaseAuthInterface
}

// GetTestContainerDBConfig returns the database configuration for the test container.
// DB container must be provisioned before calling this function.
func (t *TestBox) GetTestContainerDBConfig(ctx context.Context) (config.DatabaseConfig, error) {
	var result config.DatabaseConfig

	if t.DBContainer == nil {
		return result, fmt.Errorf("database container not initialized")
	}

	host, err := t.DBContainer.Host(ctx)
	if err != nil {
		_ = t.DBContainer.Terminate(ctx)
		return result, fmt.Errorf("failed to get container host: %w", err)
	}

	port, err := t.DBContainer.MappedPort(ctx, "5432")
	if err != nil {
		_ = t.DBContainer.Terminate(ctx)
		return result, fmt.Errorf("failed to get container port: %w", err)
	}

	return config.DatabaseConfig{
		Host:     host,
		Port:     port.Port(),
		User:     "test_user",
		Password: "test_password",
		DBName:   "test_db",
	}, nil
}

func (t *TestBox) GetConfig() {
	ctx, cancel := context.WithTimeout(context.Background(), suiteSetupStepTimeout)
	defer cancel()

	dbConfig, err := t.GetTestContainerDBConfig(ctx)
	Expect(err).NotTo(HaveOccurred(), "Failed to get database config")

	cfg := config.Load()

	t.Config = *cfg
	t.Config.Database = dbConfig
	t.Config.Migration.Directory = pkg.TranslateCallerRelativePath("../../database/migrations")

	t.Config.Casbin.ModelFile = pkg.TranslateCallerRelativePath("../../rbac_model.conf")
	t.Config.Casbin.PolicyFile = pkg.TranslateCallerRelativePath("../../rbac_policy.csv")
}

func (t *TestBox) ProvisionDB() {
	var err error

	ctx, cancel := context.WithTimeout(context.Background(), suiteProvisionTimeout)
	defer cancel()

	t.DBContainer, err = postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase(testDBName),
		postgres.WithUsername(testDBUser),
		postgres.WithPassword(testDBPassword),
		testcontainers.WithWaitStrategy(
			wait.ForAll(
				wait.ForLog("database system is ready to accept connections").
					WithOccurrence(1),
				wait.ForSQL(nat.Port(testDBPort), "postgres", func(host string, port nat.Port) string {
					return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", testDBUser, testDBPassword, host, port.Port(), testDBName)
				}).WithStartupTimeout(30*time.Second),
			).WithDeadline(60*time.Second),
		),
	)
	Expect(err).NotTo(HaveOccurred(), "Failed to provision database")
}

func (t *TestBox) DeprovisionDB() {

	// Terminate database container
	if t.DBContainer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), suiteDeprovisionTimeout)
		defer cancel()

		if err := t.DBContainer.Terminate(ctx); err != nil {
			GinkgoWriter.Printf("Failed to terminate database container: %v\n", err)
			return
		}
		GinkgoWriter.Printf("Database container terminated successfully\n")
	}
}

func (t *TestBox) InitDBConn() {
	ctx, cancel := context.WithTimeout(context.Background(), suiteSetupStepTimeout)
	defer cancel()

	dbConfig, err := t.GetTestContainerDBConfig(ctx)
	Expect(err).NotTo(HaveOccurred(), "Failed to get database config")

	// Retry connection with exponential backoff
	var db *gorm.DB
	maxRetries := 5
	for i := 0; i < maxRetries; i++ {
		db, err = database.Initialize(dbConfig)
		if err == nil {
			// Verify connection is actually working
			sqlDB, dbErr := db.DB()
			if dbErr == nil {
				if pingErr := sqlDB.Ping(); pingErr == nil {
					break
				}
			}
		}

		if i < maxRetries-1 {
			time.Sleep(time.Duration(i+1) * time.Second)
		}
	}

	t.DB = db
}

func (t *TestBox) CloseDBConn() {
	if t.DB != nil {
		sqlDB, err := t.DB.DB()
		if err == nil {
			if err := sqlDB.Close(); err != nil {
				GinkgoWriter.Printf("Failed to close database connection: %v\n", err)
				return
			}

			GinkgoWriter.Printf("Database connection closed successfully\n")
		}
	}
}

func (t *TestBox) RunMigrations() {
	if t.DB == nil {
		Fail("Database connection not established")
	}

	err := database.MigrateUp(t.DB, t.Config.Migration.Directory)
	if err != nil {
		Failf("Failed to run migrations: %v", err)
	}
}

func (t *TestBox) InitDependencies() {
	_, err := log.Init(t.Config.Log)
	Expect(err).NotTo(HaveOccurred(), "Failed to initialize logger")
}

func (t *TestBox) InitMockDependencies() {
	t.AuthMock = &authmocks.FirebaseAuthInterface{}
}

func (t *TestBox) InitAndStartServer() {
	var err error
	t.Server, err = server.SetupServer(&t.Config, t.DB, t.AuthMock)
	Expect(err).NotTo(HaveOccurred(), "Failed to setup server")

	// Find an available port for parallel test execution
	// Start from base port and increment if needed
	portNumber := testServerPortBase
	for {
		listener, err := net.Listen("tcp", fmt.Sprintf(":%d", portNumber))
		if err == nil {
			// Port is available, use it
			listener.Close()
			break
		}
		// Port is in use, try next port
		portNumber++
		if portNumber > testServerPortBase+100 {
			Fail(fmt.Sprintf("Failed to find available port after 100 attempts starting from %d", testServerPortBase))
		}
	}

	t.BaseURL = fmt.Sprintf("http://127.0.0.1:%d", portNumber)

	serverErr := make(chan error, 1)
	go func() {
		err := t.Server.Start(fmt.Sprintf(":%d", portNumber))
		if err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	time.Sleep(100 * time.Millisecond)
	select {
	case err := <-serverErr:
		Expect(err).NotTo(HaveOccurred(), "Server failed to start")
	default:
		// No immediate error, verify server is actually responsive
	}
}

func (t *TestBox) ShutdownServer() {
	if t.Server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), suiteTeardownStepTimeout)
		defer cancel()
		if err := t.Server.Shutdown(ctx); err != nil {
			GinkgoWriter.Printf("Failed to shutdown server gracefully: %v\n", err)
			return
		}
		GinkgoWriter.Printf("Server shutdown successfully\n")
	}
}

func (t *TestBox) ContextfulDB() *gorm.DB {
	ctx := t.DefaultContext
	return t.DB.WithContext(ctx)
}

func (t *TestBox) CleanUploadFiles() {
	os.RemoveAll(t.Config.UploadsBasePath)
}
