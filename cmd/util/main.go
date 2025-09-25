package main

import (
	"fmt"
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
		if err := seedDatabase(); err != nil {
			fmt.Fprintf(os.Stderr, "Error seeding database: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✓ Database seeded successfully!")
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

	rootCmd.AddCommand(authCmd)
	rootCmd.AddCommand(seedCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
