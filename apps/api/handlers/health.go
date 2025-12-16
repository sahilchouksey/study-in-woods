package handlers

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/sahilchouksey/go-init-setup/database"
)

// serverStartTime tracks when the server started for uptime calculation
var serverStartTime = time.Now()

func HandleCheckHealth(c *fiber.Ctx, store database.Storage) error {
	(*c).JSON(fiber.Map{"status": "ok"})
	return nil
}

// HandleDetailedHealth returns detailed health information including database status
func HandleDetailedHealth(c *fiber.Ctx, store database.Storage) error {
	// Check database connection
	dbStatus := "connected"
	if err := store.HealthCheck(); err != nil {
		dbStatus = "disconnected"
	}

	// Calculate uptime
	uptime := time.Since(serverStartTime)

	return c.JSON(fiber.Map{
		"status":    "ok",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"uptime":    uptime.String(),
		"database":  dbStatus,
	})
}
