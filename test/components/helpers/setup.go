package helpers

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"cim-backend/database"
	"cim-backend/internal/config"
	"cim-backend/internal/server"

	"github.com/docker/go-connections/nat"
	"github.com/labstack/echo/v4"
	_ "github.com/lib/pq"
	"github.com/sirupsen/logrus"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/gorm"
)

// TestContainer holds all test infrastructure (database, server, etc.)
type TestContainer struct {
	PostgresContainer testcontainers.Container
	DB                *gorm.DB
	Server            *echo.Echo
	BaseURL           string
	MockAuth          *MockFirebaseAuthService
	originalDir       string
	Cleanup           func()
}

// getProjectRoot returns the project root directory
func getProjectRoot() (string, error) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("failed to get current file path")
	}

	// Go up from test/components/setup.go to project root
	projectRoot := filepath.Join(filepath.Dir(filename), "..", "..", "..")
	absPath, err := filepath.Abs(projectRoot)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}
	return absPath, nil
}

// SetupTestContainers sets up testcontainers with Postgres database
func SetupTestContainers(ctx context.Context) (*TestContainer, error) {
	// Change to project root directory to ensure Casbin can find rbac_model.conf
	projectRoot, err := getProjectRoot()
	if err != nil {
		return nil, fmt.Errorf("failed to get project root: %w", err)
	}

	originalDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current directory: %w", err)
	}

	if err := os.Chdir(projectRoot); err != nil {
		return nil, fmt.Errorf("failed to change to project root: %w", err)
	}

	// Start Postgres container with SQL wait strategy to ensure it's ready
	postgresContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("test_db"),
		postgres.WithUsername("test_user"),
		postgres.WithPassword("test_password"),
		testcontainers.WithWaitStrategy(
			wait.ForAll(
				wait.ForLog("database system is ready to accept connections").
					WithOccurrence(1),
				wait.ForSQL(nat.Port("5432"), "postgres", func(host string, port nat.Port) string {
					return fmt.Sprintf("postgres://test_user:test_password@%s:%s/test_db?sslmode=disable", host, port.Port())
				}).WithStartupTimeout(30*time.Second),
			).WithDeadline(60*time.Second),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to start postgres container: %w", err)
	}

	// Get connection details
	host, err := postgresContainer.Host(ctx)
	if err != nil {
		postgresContainer.Terminate(ctx)
		return nil, fmt.Errorf("failed to get container host: %w", err)
	}

	port, err := postgresContainer.MappedPort(ctx, "5432")
	if err != nil {
		postgresContainer.Terminate(ctx)
		return nil, fmt.Errorf("failed to get container port: %w", err)
	}

	// Setup database connection with retry logic
	dbConfig := config.DatabaseConfig{
		Host:     host,
		Port:     port.Port(),
		User:     "test_user",
		Password: "test_password",
		DBName:   "test_db",
	}

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

	if err != nil {
		postgresContainer.Terminate(ctx)
		return nil, fmt.Errorf("failed to initialize database after retries: %w", err)
	}

	// Verify connection one more time
	sqlDB, err := db.DB()
	if err != nil {
		postgresContainer.Terminate(ctx)
		return nil, fmt.Errorf("failed to get database connection: %w", err)
	}

	if err := sqlDB.Ping(); err != nil {
		postgresContainer.Terminate(ctx)
		return nil, fmt.Errorf("database ping failed: %w", err)
	}

	// Run migrations
	if err := database.AutoMigrate(db); err != nil {
		postgresContainer.Terminate(ctx)
		return nil, fmt.Errorf("failed to run auto migrations: %w", err)
	}

	// Try to run SQL migrations if the directory exists
	migrationDir := "database/migrations"
	// if err := database.MigrateScripts(db, migrationDir); err != nil {
	// 	// Log but don't fail - migrations directory might not exist
	// 	fmt.Printf("Warning: Failed to run SQL migrations: %v\n", err)
	// }

	// Create mock Firebase Auth
	mockAuth := NewMockFirebaseAuthService()

	// Create test config
	cfg := &config.Config{
		Environment: "test",
		Database:    dbConfig,
		Server: config.ServerConfig{
			Host:               "127.0.0.1",
			Port:               "0", // Use 0 to get random port
			CORSAllowedOrigins: []string{"*"},
		},
		Firebase: config.FirebaseConfig{
			ServiceAccountPath: "",
			ProjectID:          "test-project",
		},
		Excel: config.ExcelConfig{
			MaxRows: 10000,
			MaxCols: 50,
		},
		Migration: config.MigrationConfig{
			Directory: migrationDir,
		},
	}

	// Setup logger
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // Reduce noise in tests

	// Setup server
	e, err := server.SetupServer(cfg, db, mockAuth, logger)
	if err != nil {
		postgresContainer.Terminate(ctx)
		return nil, fmt.Errorf("failed to setup server: %w", err)
	}

	// Find an available port
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		postgresContainer.Terminate(ctx)
		return nil, fmt.Errorf("failed to find available port: %w", err)
	}
	portNumber := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	// Start server in a goroutine
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", portNumber)
	serverErr := make(chan error, 1)
	go func() {
		if err := e.Start(fmt.Sprintf(":%d", portNumber)); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	// Wait a bit for server to start
	time.Sleep(100 * time.Millisecond)
	select {
	case err := <-serverErr:
		postgresContainer.Terminate(ctx)
		return nil, fmt.Errorf("server failed to start: %w", err)
	default:
		// Server started successfully
	}

	// Cleanup function
	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Shutdown server
		if e != nil {
			e.Shutdown(ctx)
		}

		// Terminate container
		if postgresContainer != nil {
			postgresContainer.Terminate(ctx)
		}

		// Close database connection
		if db != nil {
			sqlDB, _ := db.DB()
			if sqlDB != nil {
				sqlDB.Close()
			}
		}

		// Restore original directory
		if originalDir != "" {
			if restoreErr := os.Chdir(originalDir); restoreErr != nil {
				fmt.Printf("Warning: failed to restore original directory: %v\n", restoreErr)
			}
		}
	}

	return &TestContainer{
		PostgresContainer: postgresContainer,
		DB:                db,
		Server:            e,
		BaseURL:           baseURL,
		MockAuth:          mockAuth,
		originalDir:       originalDir,
		Cleanup:           cleanup,
	}, nil
}
