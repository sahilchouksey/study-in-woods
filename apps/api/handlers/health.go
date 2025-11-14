package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/sahilchouksey/go-init-setup/database"
)

func HandleCheckHealth(c fiber.Ctx, store database.Storage) error {
	c.JSON(fiber.Map{"status": "ok"})
	return nil
}
