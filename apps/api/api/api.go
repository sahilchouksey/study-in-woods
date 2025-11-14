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
		app:           fiber.New(),
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
