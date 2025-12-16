//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	// Load .env
	godotenv.Load()

	// Build database URL
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbUser := os.Getenv("DB_USER_NAME")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")

	if dbHost == "" {
		dbHost = "localhost"
	}
	if dbPort == "" {
		dbPort = "5432"
	}

	dbURL := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPassword, dbName)

	db, err := gorm.Open(postgres.Open(dbURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	fmt.Println("========================================")
	fmt.Println("CLEANUP: Deleting test data")
	fmt.Println("========================================")

	// Delete in correct order due to foreign key constraints

	// 1. Delete indexing job items
	result := db.Exec("DELETE FROM indexing_job_items")
	fmt.Printf("Deleted %d indexing job items\n", result.RowsAffected)

	// 2. Delete indexing jobs
	result = db.Exec("DELETE FROM indexing_jobs")
	fmt.Printf("Deleted %d indexing jobs\n", result.RowsAffected)

	// 3. Delete PYQ papers
	result = db.Exec("DELETE FROM pyq_papers")
	fmt.Printf("Deleted %d PYQ papers\n", result.RowsAffected)

	// 4. Delete PYQ-related documents
	result = db.Exec("DELETE FROM documents WHERE type = 'pyq'")
	fmt.Printf("Deleted %d PYQ documents\n", result.RowsAffected)

	// 5. Delete PYQ-related notifications
	result = db.Exec("DELETE FROM user_notifications WHERE category = 'pyq_ingest'")
	fmt.Printf("Deleted %d PYQ notifications\n", result.RowsAffected)

	fmt.Println("\n✅ Cleanup complete!")
	fmt.Println("========================================")
}
