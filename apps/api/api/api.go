package api

import (
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/sahilchouksey/go-init-setup/database"
)

type APIServer struct {
	app           *fiber.App
	listenAddress string
	store         database.Storage
}

func NewAPIServer(listenAddress string) *APIServer {
	return &APIServer{
		app: fiber.New(fiber.Config{
			// Increase body limit for file uploads (100MB)
			BodyLimit: 100 * 1024 * 1024,
			// Increase read buffer for large multipart forms
			ReadBufferSize: 8192,
		}),
		listenAddress: listenAddress,
	}
}

func (s *APIServer) GetEngine() *fiber.App {
	return s.app
}

func (s *APIServer) Run() error {
	log.Println("Starting API Server")
	log.Println("Listening on %s", s.listenAddress)

	return s.app.Listen(s.listenAddress)
}
