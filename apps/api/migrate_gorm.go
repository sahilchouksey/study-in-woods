// migrate_gorm.go - Run this file to test GORM migrations
// Usage: go run migrate_gorm.go

//go:build ignore

package main

import (
	"log"

	"github.com/sahilchouksey/go-init-setup/config"
	"github.com/sahilchouksey/go-init-setup/database"
)

func main() {
	log.Println("=== GORM Migration Test ===")

	// Load environment variables
	if err := config.LoadENV(); err != nil {
		log.Fatal("Failed to load environment variables:", err)
	}

	// Initialize GORM connection
	store, err := database.StartGORM()
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer store.Close()

	// Run migrations
	if err := store.Init(); err != nil {
		log.Fatal("Failed to run migrations:", err)
	}

	// Health check
	if err := store.HealthCheck(); err != nil {
		log.Fatal("Database health check failed:", err)
	}

	log.Println("✅ All migrations completed successfully!")
	log.Println("✅ Database connection healthy!")
	log.Println("\nYou can now check your PostgreSQL database to see the new tables:")
	log.Println("  - universities")
	log.Println("  - courses")
	log.Println("  - semesters")
	log.Println("  - subjects")
	log.Println("  - users")
	log.Println("  - user_courses")
	log.Println("  - documents")
	log.Println("  - chat_sessions")
	log.Println("  - chat_messages")
	log.Println("  - course_payments")
	log.Println("  - api_key_usage_logs")
	log.Println("  - app_settings")
	log.Println("  - jwt_token_blacklist")
	log.Println("  - cron_job_logs")
	log.Println("  - admin_audit_logs")
}
