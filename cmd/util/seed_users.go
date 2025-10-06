package main

import (
	"context"
	"import-export-backend/internal/auth"
	"import-export-backend/internal/config"
	"import-export-backend/internal/database"
	"import-export-backend/internal/models"
	"import-export-backend/internal/repository"
	"import-export-backend/internal/services"
	"log"

	"github.com/spf13/cobra"
)

var seedUsersCmd = &cobra.Command{
	Use:   "seed-users",
	Short: "Seed default users with different roles",
	Run:   seedUsers,
}

func init() {
	rootCmd.AddCommand(seedUsersCmd)
}

func seedUsers(cmd *cobra.Command, args []string) {
	// Load configuration
	cfg := config.Load()

	// Initialize database
	db, err := database.Initialize(cfg.Database)
	if err != nil {
		log.Fatal("Failed to initialize database:", err)
	}

	// Initialize Casbin service
	casbinService, err := auth.NewCasbinService(db)
	if err != nil {
		log.Fatal("Failed to initialize Casbin service:", err)
	}

	// Initialize user repository and service
	userRepo := repository.NewUserRepository(db)
	userService := services.NewUserService(userRepo, casbinService)

	ctx := context.Background()

	// Create default users
	defaultUsers := []struct {
		UID   string
		Email string
		Name  string	
		Role  string
	}{
		{
			UID:   "demoAdminUid0000000000000000",
			Email: "test@cim.local",
			Name:  "Admin User",
			Role:  string(models.RoleAdmin),
		},
		{
			UID:   "demoRootAdminUid000000000000",
			Email: "admin@example.com",
			Name:  "Admin User",
			Role:  string(models.RoleAdmin),
		},
		{
			UID:   "demoRootAdminUid000000000000",
			Email: "admin2@example.com",
			Name:  "Admin User",
			Role:  string(models.RoleAdmin),
		},

		{
			UID:   "demoAccountantUid00000000000",
			Email: "accountant@cim.local",
			Name:  "Accountant User",
			Role:  string(models.RoleAccountant),
		},
		{
			UID:   "demoStaffUid0000000000000000",
			Email: "staff@cim.local",
			Name:  "Staff User",
			Role:  string(models.RoleStaff),
		},
	}

	for _, userData := range defaultUsers {
		// Check if user already exists
		existingUser, err := userService.GetUserByUID(ctx, userData.UID)
		if err != nil {
			log.Printf("Error checking existing user %s: %v", userData.Email, err)
			continue
		}

		if existingUser != nil {
			log.Printf("User %s already exists, skipping", userData.Email)
			continue
		}

		// Create user
		user, err := userService.CreateOrUpdateUser(ctx, userData.UID, userData.Email, userData.Name)
		if err != nil {
			log.Printf("Failed to create user %s: %v", userData.Email, err)
			continue
		}

		// Update role if different from default
		if user.Role != userData.Role {
			err = userService.UpdateUserRole(ctx, user.UID, userData.Role, "system")
			if err != nil {
				log.Printf("Failed to update role for user %s: %v", userData.Email, err)
				continue
			}
		}

		log.Printf("Created user: %s with role: %s", userData.Email, userData.Role)
	}

	log.Println("User seeding completed!")
}
