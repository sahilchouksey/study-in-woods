package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/joho/godotenv"
	"github.com/sahilchouksey/go-init-setup/database"
	"gorm.io/gorm"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found, using system environment variables")
	}

	// Initialize database connection using GORM
	store, err := database.StartGORM()
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	gormDB := store.GetDB().(*gorm.DB)

	// Run seeds
	separator := strings.Repeat("=", 60)
	fmt.Println(separator)
	fmt.Println("Study in Woods - Database Seeding")
	fmt.Println(separator)
	fmt.Println()

	if err := database.RunSeeds(gormDB); err != nil {
		log.Fatalf("❌ Seeding failed: %v", err)
	}

	fmt.Println()
	fmt.Println(separator)
	fmt.Println("🎉 Seeding completed successfully!")
	fmt.Println(separator)
	fmt.Println()
	fmt.Println("Default Admin Credentials:")
	fmt.Println("  Email:    admin@studyinwoods.com")
	fmt.Println("  Password: Admin123!")
	fmt.Println()
	fmt.Println("⚠️  Please change the admin password after first login!")
	fmt.Println()
}
