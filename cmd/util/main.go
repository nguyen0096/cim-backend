package main

import (
	"fmt"
	"import-export-backend/test/data"
	"os"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

var (
	apiKey          string
	defaultUsername string
	defaultPassword string
)

var rootCmd = &cobra.Command{
	Use:   "util",
	Short: "Utility CLI for import-export-backend",
	Long:  "A utility CLI application for various import-export-backend operations",
}

var seedCmd = &cobra.Command{
	Use:   "seed",
	Short: "Seed the database with mock data",
	Long:  "Populate the database with predefined mock data for manual testing",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("🌱 Seeding database with mock data...")
		if err := data.SeedDatabase(); err != nil {
			fmt.Fprintf(os.Stderr, "Error seeding database: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✓ Database seeded successfully!")
	},
}

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Database migration commands",
	Long:  "Run database migrations to create, update, or rollback the database schema",
	Run: func(cmd *cobra.Command, args []string) {
		// Default to migrate up if no subcommand is provided
		fmt.Println("🔄 Running database migrations...")
		if err := runMigrations(); err != nil {
			fmt.Fprintf(os.Stderr, "Error running migrations: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✓ Database migrations completed successfully!")
	},
}

var migrateUpCmd = &cobra.Command{
	Use:   "up",
	Short: "Run database migrations",
	Long:  "Run database migrations to create or update the database schema",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("🔄 Running database migrations...")
		if err := runMigrations(); err != nil {
			fmt.Fprintf(os.Stderr, "Error running migrations: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✓ Database migrations completed successfully!")
	},
}

var migrateDownCmd = &cobra.Command{
	Use:   "down",
	Short: "Rollback database migrations",
	Long:  "Drop all database tables (complete rollback)",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("⚠️  Rolling back database migrations (dropping all tables)...")
		if err := rollbackMigrations(); err != nil {
			fmt.Fprintf(os.Stderr, "Error rolling back migrations: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✓ Database rollback completed successfully!")
	},
}

func init() {
	// Load environment variables from .env file
	if err := godotenv.Load(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Could not load .env file: %v\n", err)
	}

	// Load Firebase environment variables (only needed for auth command)
	apiKey = os.Getenv("FIREBASE_WEB_API_KEY")
	defaultUsername = os.Getenv("FIREBASE_TEST_USER")
	defaultPassword = os.Getenv("FIREBASE_TEST_PASSWORD")

	// Add subcommands to migrate command
	migrateCmd.AddCommand(migrateUpCmd)
	migrateCmd.AddCommand(migrateDownCmd)

	rootCmd.AddCommand(authCmd)
	rootCmd.AddCommand(seedCmd)
	rootCmd.AddCommand(migrateCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
