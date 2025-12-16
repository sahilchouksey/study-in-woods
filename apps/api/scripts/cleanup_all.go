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

// This script deletes ALL:
// 1. Agents from DigitalOcean
// 2. Knowledge Bases from DigitalOcean
// 3. Subject dependencies from database (PYQ papers, indexing jobs, syllabi, units, topics)
// 4. Subjects from database
//
// Usage: go run scripts/cleanup_all.go [--dry-run]
// WARNING: This is destructive and cannot be undone!

func main() {
	dryRun := false
	for _, arg := range os.Args[1:] {
		if arg == "--dry-run" {
			dryRun = true
		}
	}

	log.Println("╔══════════════════════════════════════════════════════════════════╗")
	log.Println("║  CLEANUP SCRIPT - Delete ALL Subjects and DO Resources           ║")
	log.Println("╚══════════════════════════════════════════════════════════════════╝")

	if dryRun {
		log.Println("\n⚠️  DRY RUN MODE - No changes will be made")
	} else {
		log.Println("\n🚨 WARNING: This will DELETE all subjects, KBs, and Agents!")
		log.Println("   Press Ctrl+C within 5 seconds to cancel...")
		time.Sleep(5 * time.Second)
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
	log.Println("✓ Connected to database")

	// Initialize DigitalOcean client
	doToken := os.Getenv("DIGITALOCEAN_TOKEN")
	if doToken == "" {
		log.Fatal("DIGITALOCEAN_TOKEN environment variable is required")
	}
	doClient := digitalocean.NewClient(digitalocean.Config{APIToken: doToken})
	log.Println("✓ Initialized DigitalOcean client")

	ctx := context.Background()

	// Step 1: Get all subjects with KB/Agent UUIDs
	log.Println("\n[STEP 1] Finding subjects with AI resources...")
	var subjects []model.Subject
	err = db.Where("knowledge_base_uuid != '' OR agent_uuid != ''").Find(&subjects).Error
	if err != nil {
		log.Fatalf("Failed to query subjects: %v", err)
	}
	log.Printf("Found %d subjects with AI resources", len(subjects))

	// Collect unique UUIDs
	agentUUIDs := make([]string, 0)
	kbUUIDs := make([]string, 0)
	subjectIDs := make([]uint, 0)

	for _, s := range subjects {
		subjectIDs = append(subjectIDs, s.ID)
		if s.AgentUUID != "" {
			agentUUIDs = append(agentUUIDs, s.AgentUUID)
		}
		if s.KnowledgeBaseUUID != "" {
			kbUUIDs = append(kbUUIDs, s.KnowledgeBaseUUID)
		}
	}

	// Step 2: Delete Agents from DigitalOcean
	log.Println("\n[STEP 2] Deleting Agents from DigitalOcean...")
	agentDeleteSuccess := 0
	agentDeleteFail := 0
	for _, agentUUID := range agentUUIDs {
		if dryRun {
			log.Printf("  [DRY RUN] Would delete agent: %s", agentUUID)
			continue
		}
		err := doClient.DeleteAgent(ctx, agentUUID)
		if err != nil {
			log.Printf("  ❌ Failed to delete agent %s: %v", agentUUID, err)
			agentDeleteFail++
		} else {
			log.Printf("  ✓ Deleted agent: %s", agentUUID)
			agentDeleteSuccess++
		}
	}
	if !dryRun {
		log.Printf("  Agents: %d deleted, %d failed", agentDeleteSuccess, agentDeleteFail)
	}

	// Step 3: Delete Knowledge Bases from DigitalOcean
	log.Println("\n[STEP 3] Deleting Knowledge Bases from DigitalOcean...")
	kbDeleteSuccess := 0
	kbDeleteFail := 0
	for _, kbUUID := range kbUUIDs {
		if dryRun {
			log.Printf("  [DRY RUN] Would delete KB: %s", kbUUID)
			continue
		}
		err := doClient.DeleteKnowledgeBase(ctx, kbUUID)
		if err != nil {
			log.Printf("  ❌ Failed to delete KB %s: %v", kbUUID, err)
			kbDeleteFail++
		} else {
			log.Printf("  ✓ Deleted KB: %s", kbUUID)
			kbDeleteSuccess++
		}
	}
	if !dryRun {
		log.Printf("  Knowledge Bases: %d deleted, %d failed", kbDeleteSuccess, kbDeleteFail)
	}

	// Step 4: Delete database dependencies
	log.Println("\n[STEP 4] Deleting database dependencies...")

	if dryRun {
		// Count what would be deleted
		var pyqCount, indexingCount, syllabiCount, notifCount, docCount int64
		db.Model(&model.PYQPaper{}).Count(&pyqCount)
		db.Model(&model.IndexingJob{}).Count(&indexingCount)
		db.Model(&model.Syllabus{}).Count(&syllabiCount)
		db.Model(&model.UserNotification{}).Count(&notifCount)
		db.Model(&model.Document{}).Count(&docCount)
		log.Printf("  [DRY RUN] Would delete: %d PYQ papers, %d indexing jobs, %d syllabi, %d notifications, %d documents", pyqCount, indexingCount, syllabiCount, notifCount, docCount)
	} else {
		// Delete PYQ question choices (child of PYQ questions)
		result := db.Exec("DELETE FROM pyq_question_choices")
		log.Printf("  ✓ Deleted %d PYQ question choices", result.RowsAffected)

		// Delete PYQ questions (child of PYQ papers)
		result = db.Exec("DELETE FROM pyq_questions")
		log.Printf("  ✓ Deleted %d PYQ questions", result.RowsAffected)

		// Delete PYQ papers
		result = db.Exec("DELETE FROM pyq_papers")
		log.Printf("  ✓ Deleted %d PYQ papers", result.RowsAffected)

		// Delete indexing job items (child of indexing jobs)
		result = db.Exec("DELETE FROM indexing_job_items")
		log.Printf("  ✓ Deleted %d indexing job items", result.RowsAffected)

		// Delete indexing jobs
		result = db.Exec("DELETE FROM indexing_jobs")
		log.Printf("  ✓ Deleted %d indexing jobs", result.RowsAffected)

		// Delete user notifications
		result = db.Exec("DELETE FROM user_notifications")
		log.Printf("  ✓ Deleted %d user notifications", result.RowsAffected)

		// Delete documents
		result = db.Exec("DELETE FROM documents")
		log.Printf("  ✓ Deleted %d documents", result.RowsAffected)

		// Delete topics (child of units)
		result = db.Exec("DELETE FROM topics")
		log.Printf("  ✓ Deleted %d topics", result.RowsAffected)

		// Delete units (child of syllabi)
		result = db.Exec("DELETE FROM units")
		log.Printf("  ✓ Deleted %d units", result.RowsAffected)

		// Delete syllabi
		result = db.Exec("DELETE FROM syllabi")
		log.Printf("  ✓ Deleted %d syllabi", result.RowsAffected)

		// Delete chat messages (child of chat sessions)
		result = db.Exec("DELETE FROM chat_messages")
		log.Printf("  ✓ Deleted %d chat messages", result.RowsAffected)

		// Delete chat sessions
		result = db.Exec("DELETE FROM chat_sessions")
		log.Printf("  ✓ Deleted %d chat sessions", result.RowsAffected)

		// Delete chats
		result = db.Exec("DELETE FROM chats")
		log.Printf("  ✓ Deleted %d chats", result.RowsAffected)
	}

	// Step 5: Delete all subjects
	log.Println("\n[STEP 5] Deleting subjects from database...")
	if dryRun {
		var subjectCount int64
		db.Model(&model.Subject{}).Count(&subjectCount)
		log.Printf("  [DRY RUN] Would delete %d subjects", subjectCount)
	} else {
		result := db.Exec("DELETE FROM subjects")
		log.Printf("  ✓ Deleted %d subjects", result.RowsAffected)
	}

	// Summary
	log.Println("\n╔══════════════════════════════════════════════════════════════════╗")
	log.Println("║  CLEANUP SUMMARY                                                  ║")
	log.Println("╚══════════════════════════════════════════════════════════════════╝")
	if dryRun {
		log.Println("  Mode: DRY RUN (no changes made)")
		log.Printf("  Would delete: %d agents, %d knowledge bases", len(agentUUIDs), len(kbUUIDs))
	} else {
		log.Printf("  Agents deleted: %d (failed: %d)", agentDeleteSuccess, agentDeleteFail)
		log.Printf("  Knowledge Bases deleted: %d (failed: %d)", kbDeleteSuccess, kbDeleteFail)
		log.Println("  Database tables cleaned: subjects, syllabi, units, topics, pyq_papers, indexing_jobs, chats, chat_sessions")
		log.Println("\n  ✅ CLEANUP COMPLETE!")
	}
}

func connectToDB() (*gorm.DB, error) {
	dbHost := getEnvVal("DB_HOST", "localhost")
	dbPort := getEnvVal("DB_PORT", "5432")
	dbUser := getEnvVal("DB_USER_NAME", "postgres")
	dbPassword := getEnvVal("DB_PASSWORD", "postgres")
	dbName := getEnvVal("DB_NAME", "study_in_woods")
	dbSSLMode := getEnvVal("DB_SSL_MODE", "disable")

	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
		dbHost, dbUser, dbPassword, dbName, dbPort, dbSSLMode,
	)

	return gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
}

func getEnvVal(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
