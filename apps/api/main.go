package main

import (
	"github.com/gofiber/fiber/v2/log"
	"github.com/sahilchouksey/go-init-setup/app"
)

func main() {
	// setup and run app
	err := app.SetupAndRunServer()
	if err != nil {
		log.Trace(err)
		panic(err)
	}
}
