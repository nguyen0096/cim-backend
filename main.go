package main

import (
	"cim-backend/database"
	"cim-backend/internal/auth"
	"cim-backend/internal/config"
	"cim-backend/internal/server"
	"log"

	"github.com/sirupsen/logrus"
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
	// Load configuration
	cfg := config.Load()

	// Initialize logger
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)
	logger.Infof("Starting application in %s environment", cfg.Environment)

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
	e, err := server.SetupServer(cfg, db, firebaseAuth, logger)
	if err != nil {
		log.Fatal("Failed to setup server:", err)
	}

	// Start server
	logger.Infof("Server starting on %s:%s", cfg.Server.Host, cfg.Server.Port)
	if err := e.Start(cfg.Server.Host + ":" + cfg.Server.Port); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}
