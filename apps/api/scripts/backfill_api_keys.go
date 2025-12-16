//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/sahilchouksey/go-init-setup/model"
	"github.com/sahilchouksey/go-init-setup/services/digitalocean"
	"github.com/sahilchouksey/go-init-setup/utils/crypto"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// This script backfills API keys for subjects that have Agents but no stored API key
// Usage: go run scripts/backfill_api_keys.go [--dry-run]

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

	// Verify ENCRYPTION_KEY is set
	if os.Getenv("ENCRYPTION_KEY") == "" {
		log.Fatal("ENCRYPTION_KEY environment variable is required")
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

	// Find subjects with Agent UUID but no encrypted API key
	var subjects []model.Subject
	err = db.Where("agent_uuid != '' AND agent_uuid IS NOT NULL AND (agent_api_key_encrypted = '' OR agent_api_key_encrypted IS NULL)").
		Find(&subjects).Error
	if err != nil {
		log.Fatalf("Failed to query subjects: %v", err)
	}

	log.Printf("Found %d subjects needing API key backfill", len(subjects))

	if len(subjects) == 0 {
		log.Println("No subjects need API key backfill. Exiting.")
		return
	}

	successCount := 0
	failCount := 0

	for _, subject := range subjects {
		log.Printf("\n--- Processing Subject: %s (ID: %d, Code: %s) ---", subject.Name, subject.ID, subject.Code)
		log.Printf("  Agent UUID: %s", subject.AgentUUID)

		if dryRun {
			log.Println("  [DRY RUN] Would create API key and encrypt it")
			continue
		}

		// Check if agent exists and is deployed
		agent, err := doClient.GetAgent(ctx, subject.AgentUUID)
		if err != nil {
			log.Printf("  ERROR: Failed to get agent: %v", err)
			failCount++
			continue
		}

		if agent.Deployment == nil || agent.Deployment.URL == "" {
			log.Printf("  WARNING: Agent not deployed yet. Attempting to deploy...")
			// Try to deploy agent (VisibilityPrivate for API key access)
			_, err := doClient.DeployAgent(ctx, subject.AgentUUID, digitalocean.VisibilityPrivate)
			if err != nil {
				log.Printf("  ERROR: Failed to deploy agent: %v", err)
				failCount++
				continue
			}
			// Wait a bit for deployment
			log.Println("  Waiting 30s for deployment...")
			time.Sleep(30 * time.Second)
		}

		// Create API key
		apiKeyName := fmt.Sprintf("%s-backfill-%d", strings.ToLower(strings.ReplaceAll(subject.Code, " ", "-")), time.Now().Unix())
		apiKeyResult, err := doClient.CreateAgentAPIKey(ctx, subject.AgentUUID, apiKeyName)
		if err != nil {
			log.Printf("  ERROR: Failed to create API key: %v", err)
			failCount++
			continue
		}

		if apiKeyResult.SecretKey == "" {
			log.Printf("  ERROR: API key created but no secret key returned")
			failCount++
			continue
		}

		// Encrypt the API key
		encryptedKey, err := crypto.EncryptAPIKeyForStorage(apiKeyResult.SecretKey)
		if err != nil {
			log.Printf("  ERROR: Failed to encrypt API key: %v", err)
			failCount++
			continue
		}

		// Update the subject
		err = db.Model(&subject).Update("agent_api_key_encrypted", encryptedKey).Error
		if err != nil {
			log.Printf("  ERROR: Failed to save encrypted key: %v", err)
			failCount++
			continue
		}

		log.Printf("  SUCCESS: API key created and stored (encrypted length: %d)", len(encryptedKey))
		successCount++
	}

	log.Println("\n=== SUMMARY ===")
	log.Printf("Total subjects processed: %d", len(subjects))
	log.Printf("Successful: %d", successCount)
	log.Printf("Failed: %d", failCount)
	if dryRun {
		log.Println("(Dry run - no actual changes were made)")
	}
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
