package main

import (
	"cim-backend/database"
	"cim-backend/internal/auth"
	"cim-backend/internal/config"
	"cim-backend/internal/server"
	"cim-backend/pkg"
	"cim-backend/pkg/log"
	"context"
	"os"
	"os/signal"
)

// @title Import Export Backend API
// @version 1.0
// @description This is an Import Export Backend API server for managing products, inventory, suppliers, and orders.
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.url http://www.swagger.io/support
// @contact.email support@swagger.io

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:8080
// @BasePath /api/v1

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Load configuration
	cfg := config.Load()

	// Initialize logger
	log.MustInit(cfg.Log)

	// Initialize OpenTelemetry
	otelShutdown, err := pkg.SetupOTelSDK(ctx)
	if err != nil {
		log.Fatal("Failed to initialize OpenTelemetry:", err)
	}
	defer func() {
		if err := otelShutdown(context.Background()); err != nil {
			log.Error("Failed to shutdown OpenTelemetry:", err)
		}
	}()

	log.Infof("Starting application in %s environment", cfg.Environment)

	// Initialize Firebase Auth
	firebaseAuth, err := auth.NewFirebaseAuthService(cfg.Firebase.ServiceAccountPath)
	if err != nil {
		log.Fatal("Failed to initialize Firebase Auth:", err)
	}

	// Initialize database
	db, err := database.Initialize(cfg.Database)
	if err != nil {
		log.Fatal("Failed to initialize database:", err)
	}

	// Setup server with all routes and middleware
	e, err := server.SetupServer(cfg, db, firebaseAuth)
	if err != nil {
		log.Fatal("Failed to setup server:", err)
	}
	defer func() {
		if err := e.Shutdown(context.Background()); err != nil {
			log.Error("Failed to shutdown server:", err)
		}
	}()

	// Start server
	srvErr := make(chan error, 1)
	go func() {
		log.Infof("Server starting on %s:%s", cfg.Server.Host, cfg.Server.Port)
		if err := e.Start(cfg.Server.Host + ":" + cfg.Server.Port); err != nil {
			srvErr <- err
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server.
	select {
	case <-ctx.Done():
		log.Info("Shutting down server...")
	case err := <-srvErr:
		log.Fatal("Failed to start server:", err)
	}
}
