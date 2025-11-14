package utils

import (
	"log"

	fiber "github.com/gofiber/fiber/v2"
	"github.com/sahilchouksey/go-init-setup/database"
)

func MakeHTTPHandleFunc(handler func(c *fiber.Ctx, store database.Storage) error, store database.Storage) func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		if err := handler(c, store); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})

		}
		return nil
	}
}

func WriteJSON(c *fiber.Ctx, status int, data interface{}, headers map[string]string, err error) {
	if err != nil {
		c.JSON(fiber.Map{"error": err.Error()})
		return
	}

	if len(headers) > 0 {
		for key, value := range headers {
			log.Println(key, value)

			//
		}
	} else {
		// c.Header("Content-Type", "application/json")
	}

	c.JSON(fiber.Map{"data": data})
}
