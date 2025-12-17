package main

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/sahilchouksey/go-init-setup/model"
	"github.com/sahilchouksey/go-init-setup/utils/auth"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// UserCredentials holds user info for display
type UserCredentials struct {
	Email    string
	Password string
	Name     string
	Role     string
}

func main() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// Connect to database
	db, err := connectDB()
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Create users
	users, err := createUsers(db)
	if err != nil {
		log.Fatalf("Failed to create users: %v", err)
	}

	// Print credentials
	printCredentials(users)
}

func connectDB() (*gorm.DB, error) {
	dbHost := getEnv("DB_HOST", "localhost")
	dbPort := getEnv("DB_PORT", "5432")
	dbUser := getEnv("DB_USER_NAME", "postgres")
	dbPassword := getEnv("DB_PASSWORD", "postgres")
	dbName := getEnv("DB_NAME", "study_in_woods")
	dbSSLMode := getEnv("DB_SSL_MODE", "disable")

	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		dbHost, dbPort, dbUser, dbPassword, dbName, dbSSLMode,
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, err
	}

	return db, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func createUsers(db *gorm.DB) ([]UserCredentials, error) {
	var credentials []UserCredentials

	// Get admin credentials from environment
	adminEmail := os.Getenv("ADMIN_EMAIL")
	adminPassword := os.Getenv("ADMIN_PASSWORD")

	if adminEmail == "" {
		adminEmail = "admin@example.com"
	}
	if adminPassword == "" {
		adminPassword = "ChangeMe123!"
	}

	// Define users to create
	usersToCreate := []struct {
		Email    string
		Password string
		Name     string
		Role     string
		Semester int
	}{
		{
			Email:    adminEmail,
			Password: adminPassword,
			Name:     "System Administrator",
			Role:     "admin",
			Semester: 0,
		},
		{
			Email:    "user@example.com",
			Password: "User123!",
			Name:     "Test Student",
			Role:     "student",
			Semester: 1,
		},
	}

	for _, u := range usersToCreate {
		// Check if user already exists
		var existingUser model.User
		result := db.Where("email = ?", u.Email).First(&existingUser)

		if result.Error == nil {
			// User exists, add to credentials list
			log.Printf("User %s already exists, skipping creation\n", u.Email)
			credentials = append(credentials, UserCredentials{
				Email:    u.Email,
				Password: u.Password,
				Name:     u.Name,
				Role:     u.Role,
			})
			continue
		}

		// Hash password
		passwordHash, err := auth.HashPassword(u.Password)
		if err != nil {
			return nil, fmt.Errorf("failed to hash password for %s: %w", u.Email, err)
		}

		// Create user
		user := &model.User{
			Email:        u.Email,
			PasswordHash: passwordHash,
			PasswordSalt: []byte("legacy_salt"), // bcrypt handles salt internally
			Name:         u.Name,
			Role:         u.Role,
			Semester:     u.Semester,
			TokenVersion: 0,
		}

		if err := db.Create(user).Error; err != nil {
			return nil, fmt.Errorf("failed to create user %s: %w", u.Email, err)
		}

		log.Printf("Created user: %s (%s)\n", u.Email, u.Role)
		credentials = append(credentials, UserCredentials{
			Email:    u.Email,
			Password: u.Password,
			Name:     u.Name,
			Role:     u.Role,
		})
	}

	return credentials, nil
}

func printCredentials(users []UserCredentials) {
	fmt.Println()
	fmt.Println("╔════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                    USER CREDENTIALS                            ║")
	fmt.Println("╠════════════════════════════════════════════════════════════════╣")

	for _, u := range users {
		roleDisplay := u.Role
		if u.Role == "admin" {
			roleDisplay = "ADMIN"
		} else {
			roleDisplay = "STUDENT"
		}

		fmt.Println("║                                                                ║")
		fmt.Printf("║  [%s]                                                     ║\n", roleDisplay)
		fmt.Printf("║  Name:     %-50s ║\n", u.Name)
		fmt.Printf("║  Email:    %-50s ║\n", u.Email)
		fmt.Printf("║  Password: %-50s ║\n", u.Password)
		fmt.Println("║                                                                ║")
		fmt.Println("╠────────────────────────────────────────────────────────────────╣")
	}

	fmt.Println("║                                                                ║")
	fmt.Println("║  Use these credentials to log in to the application.          ║")
	fmt.Println("║                                                                ║")
	fmt.Println("╚════════════════════════════════════════════════════════════════╝")
	fmt.Println()
}
