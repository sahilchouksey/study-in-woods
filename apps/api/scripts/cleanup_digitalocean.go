//go:build ignore
// +build ignore

package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/sahilchouksey/go-init-setup/services/digitalocean"
)

// This script deletes ALL resources from DigitalOcean GradientAI:
// 1. All Agents (directly from DO API)
// 2. All Knowledge Bases (directly from DO API)
//
// Usage: go run scripts/cleanup_digitalocean.go [--dry-run]
// WARNING: This is destructive and cannot be undone!

func main() {
	dryRun := false
	for _, arg := range os.Args[1:] {
		if arg == "--dry-run" {
			dryRun = true
		}
	}

	log.Println("===============================================================")
	log.Println("  DIGITALOCEAN CLEANUP - Delete ALL KBs and Agents from DO API")
	log.Println("===============================================================")

	if dryRun {
		log.Println("\n[DRY RUN MODE] - No changes will be made")
	} else {
		log.Println("\n[WARNING] This will DELETE all knowledge bases and agents from DigitalOcean!")
		log.Println("Press Ctrl+C within 5 seconds to cancel...")
		time.Sleep(5 * time.Second)
	}

	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// Initialize DigitalOcean client
	doToken := os.Getenv("DIGITALOCEAN_TOKEN")
	if doToken == "" {
		log.Fatal("DIGITALOCEAN_TOKEN environment variable is required")
	}
	doClient := digitalocean.NewClient(digitalocean.Config{APIToken: doToken})
	log.Println("[OK] Initialized DigitalOcean client")

	ctx := context.Background()

	// ===== STEP 1: List and Delete ALL Agents =====
	log.Println("\n[STEP 1] Listing all agents from DigitalOcean...")

	allAgents := []digitalocean.Agent{}
	page := 1
	for {
		agents, pagination, err := doClient.ListAgents(ctx, &digitalocean.ListOptions{
			Page:    page,
			PerPage: 100,
		})
		if err != nil {
			log.Fatalf("Failed to list agents: %v", err)
		}
		allAgents = append(allAgents, agents...)

		if pagination == nil || page >= pagination.TotalPages {
			break
		}
		page++
	}

	log.Printf("Found %d agents to delete", len(allAgents))

	agentDeleteSuccess := 0
	agentDeleteFail := 0
	for _, agent := range allAgents {
		if dryRun {
			log.Printf("  [DRY RUN] Would delete agent: %s (%s)", agent.Name, agent.UUID)
			continue
		}
		err := doClient.DeleteAgent(ctx, agent.UUID)
		if err != nil {
			log.Printf("  [FAIL] Failed to delete agent %s (%s): %v", agent.Name, agent.UUID, err)
			agentDeleteFail++
		} else {
			log.Printf("  [OK] Deleted agent: %s (%s)", agent.Name, agent.UUID)
			agentDeleteSuccess++
		}
		// Small delay to avoid rate limiting
		time.Sleep(200 * time.Millisecond)
	}
	if !dryRun && len(allAgents) > 0 {
		log.Printf("Agents: %d deleted, %d failed", agentDeleteSuccess, agentDeleteFail)
	}

	// ===== STEP 2: List and Delete ALL Knowledge Bases =====
	log.Println("\n[STEP 2] Listing all knowledge bases from DigitalOcean...")

	allKBs := []digitalocean.KnowledgeBase{}
	page = 1
	for {
		kbs, pagination, err := doClient.ListKnowledgeBases(ctx, &digitalocean.ListOptions{
			Page:    page,
			PerPage: 100,
		})
		if err != nil {
			log.Fatalf("Failed to list knowledge bases: %v", err)
		}
		allKBs = append(allKBs, kbs...)

		if pagination == nil || page >= pagination.TotalPages {
			break
		}
		page++
	}

	log.Printf("Found %d knowledge bases to delete", len(allKBs))

	kbDeleteSuccess := 0
	kbDeleteFail := 0
	for _, kb := range allKBs {
		if dryRun {
			log.Printf("  [DRY RUN] Would delete KB: %s (%s)", kb.Name, kb.UUID)
			continue
		}
		err := doClient.DeleteKnowledgeBase(ctx, kb.UUID)
		if err != nil {
			log.Printf("  [FAIL] Failed to delete KB %s (%s): %v", kb.Name, kb.UUID, err)
			kbDeleteFail++
		} else {
			log.Printf("  [OK] Deleted KB: %s (%s)", kb.Name, kb.UUID)
			kbDeleteSuccess++
		}
		// Small delay to avoid rate limiting
		time.Sleep(200 * time.Millisecond)
	}
	if !dryRun && len(allKBs) > 0 {
		log.Printf("Knowledge Bases: %d deleted, %d failed", kbDeleteSuccess, kbDeleteFail)
	}

	// ===== Summary =====
	log.Println("\n===============================================================")
	log.Println("  CLEANUP SUMMARY")
	log.Println("===============================================================")
	if dryRun {
		log.Println("Mode: DRY RUN (no changes made)")
		log.Printf("Would delete: %d agents, %d knowledge bases", len(allAgents), len(allKBs))
	} else {
		log.Printf("Agents deleted: %d (failed: %d)", agentDeleteSuccess, agentDeleteFail)
		log.Printf("Knowledge Bases deleted: %d (failed: %d)", kbDeleteSuccess, kbDeleteFail)
		if agentDeleteFail == 0 && kbDeleteFail == 0 {
			log.Println("\n[DONE] CLEANUP COMPLETE!")
		} else {
			log.Println("\n[DONE] Cleanup completed with some failures")
		}
	}
}
