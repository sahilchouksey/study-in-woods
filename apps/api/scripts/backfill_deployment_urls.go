//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/sahilchouksey/go-init-setup/model"
	"github.com/sahilchouksey/go-init-setup/services/digitalocean"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// This script backfills deployment URLs for subjects that have Agents but no deployment URL
// Usage: go run scripts/backfill_deployment_urls.go [--dry-run]

func main() {
	dryRun := false
	for _, arg := range os.Args[1:] {
		if arg == "--dry-run" {
			dryRun = true
		}
	}

	if dryRun {
		log.Println("=== DRY RUN MODE - No changes will be made ===")
	}

	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// Connect to database
	db, err := connectToDB()
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Initialize DigitalOcean client
	doToken := os.Getenv("DIGITALOCEAN_TOKEN")
	if doToken == "" {
		log.Fatal("DIGITALOCEAN_TOKEN environment variable is required")
	}
	doClient := digitalocean.NewClient(digitalocean.Config{APIToken: doToken})

	ctx := context.Background()

	// Find subjects with Agent UUID but no deployment URL
	var subjects []model.Subject
	err = db.Where("agent_uuid != '' AND agent_uuid IS NOT NULL AND (agent_deployment_url = '' OR agent_deployment_url IS NULL)").
		Find(&subjects).Error
	if err != nil {
		log.Fatalf("Failed to query subjects: %v", err)
	}

	log.Printf("Found %d subjects needing deployment URL backfill", len(subjects))

	if len(subjects) == 0 {
		log.Println("No subjects need deployment URL backfill. Exiting.")
		return
	}

	successCount := 0
	failCount := 0

	for _, subject := range subjects {
		log.Printf("\n--- Processing Subject: %s (ID: %d) ---", subject.Name, subject.ID)
		log.Printf("  Agent UUID: %s", subject.AgentUUID)

		if dryRun {
			log.Println("  [DRY RUN] Would fetch deployment URL from DigitalOcean")
			continue
		}

		// Get agent details from DigitalOcean
		agent, err := doClient.GetAgent(ctx, subject.AgentUUID)
		if err != nil {
			log.Printf("  ERROR: Failed to get agent: %v", err)
			failCount++
			continue
		}

		// Check if agent has deployment
		if agent.Deployment == nil || agent.Deployment.URL == "" {
			log.Printf("  WARNING: Agent not deployed. Attempting to deploy...")

			// Try to deploy the agent
			_, err := doClient.DeployAgent(ctx, subject.AgentUUID, digitalocean.VisibilityPrivate)
			if err != nil {
				log.Printf("  ERROR: Failed to deploy agent: %v", err)
				failCount++
				continue
			}

			// Wait for deployment to provision
			log.Println("  Waiting 10s for deployment to provision...")
			time.Sleep(10 * time.Second)

			// Re-fetch to get the URL
			agent, err = doClient.GetAgent(ctx, subject.AgentUUID)
			if err != nil {
				log.Printf("  ERROR: Failed to re-fetch agent after deployment: %v", err)
				failCount++
				continue
			}

			if agent.Deployment == nil || agent.Deployment.URL == "" {
				log.Printf("  ERROR: Agent still has no deployment URL after deploy")
				failCount++
				continue
			}
		}

		deploymentURL := agent.Deployment.URL
		log.Printf("  Found deployment URL: %s", deploymentURL)

		// Update subject in database
		if err := db.Model(&subject).Update("agent_deployment_url", deploymentURL).Error; err != nil {
			log.Printf("  ERROR: Failed to update subject: %v", err)
			failCount++
			continue
		}

		log.Printf("  SUCCESS: Updated subject %d with deployment URL", subject.ID)
		successCount++
	}

	fmt.Println()
	fmt.Println("========================================")
	fmt.Printf("Backfill complete!\n")
	fmt.Printf("  Success: %d\n", successCount)
	fmt.Printf("  Failed:  %d\n", failCount)
	fmt.Printf("  Total:   %d\n", len(subjects))
	fmt.Println("========================================")
}

func connectToDB() (*gorm.DB, error) {
	dbHost := getEnvValue("DB_HOST", "localhost")
	dbPort := getEnvValue("DB_PORT", "5432")
	dbUser := getEnvValue("DB_USER_NAME", "postgres")
	dbPassword := getEnvValue("DB_PASSWORD", "postgres")
	dbName := getEnvValue("DB_NAME", "study_in_woods")
	dbSSLMode := getEnvValue("DB_SSL_MODE", "disable")

	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
		dbHost, dbUser, dbPassword, dbName, dbPort, dbSSLMode,
	)

	return gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
}

func getEnvValue(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
