package config

import (
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Database     DatabaseConfig
	Server       ServerConfig
	JWT          JWTConfig
	Firebase     FirebaseConfig
	Excel        ExcelConfig
	Migration    MigrationConfig
	GoogleSheets GoogleSheetsConfig
}

type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
}

type ServerConfig struct {
	Host               string
	Port               string
	CORSAllowedOrigins []string
}

type JWTConfig struct {
	Secret string
}

type FirebaseConfig struct {
	ServiceAccountPath string
	ProjectID          string
}

type ExcelConfig struct {
	MaxRows int
	MaxCols int
}

type MigrationConfig struct {
	Directory string
}

type GoogleSheetsConfig struct {
	ServiceAccountPath     string
	InventorySpreadsheetID string
}

// Load loads configuration from environment variables.
// Optional filePath parameter can be provided to load a specific .env file.
// If no filePath is provided, it attempts to load .env from the current directory.
func Load(filePath ...string) *Config {
	godotenv.Load(filePath...)

	return &Config{
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "postgres"),
			Password: getEnv("DB_PASSWORD", "password"),
			DBName:   getEnv("DB_NAME", "cim_db"),
		},
		Server: ServerConfig{
			Host:               getEnv("SERVER_HOST", "0.0.0.0"),
			Port:               getEnv("SERVER_PORT", "8080"),
			CORSAllowedOrigins: getEnvAsSlice("CORS_ALLOWED_ORIGINS", []string{""}),
		},
		Firebase: FirebaseConfig{
			ServiceAccountPath: getEnv("FIREBASE_SERVICE_ACCOUNT_PATH", "./firebase-service-account.json"),
			ProjectID:          getEnv("FIREBASE_PROJECT_ID", "cafeteria-inventory-management"),
		},
		Excel: ExcelConfig{
			MaxRows: getEnvAsInt("EXCEL_MAX_ROWS", 10000),
			MaxCols: getEnvAsInt("EXCEL_MAX_COLS", 50),
		},
		Migration: MigrationConfig{
			Directory: getEnv("MIGRATION_DIR", "database/migrations"),
		},
		GoogleSheets: GoogleSheetsConfig{
			ServiceAccountPath:     getEnv("GOOGLE_SERVICE_ACCOUNT", ""),
			InventorySpreadsheetID: getEnv("INVENTORY_SPREADSHEET_ID", ""),
		},
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvAsSlice(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		parts := strings.Split(value, ",")
		result := make([]string, 0, len(parts))
		for _, part := range parts {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				result = append(result, trimmed)
			}
		}
		return result
	}
	return defaultValue
}
