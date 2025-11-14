package app

import (
	"fmt"
	"os"

	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/sahilchouksey/go-init-setup/api"
	"github.com/sahilchouksey/go-init-setup/config"
	"github.com/sahilchouksey/go-init-setup/router"
	"github.com/sahilchouksey/go-init-setup/services/cron"
	"gorm.io/gorm"

	//"github.com/sahilchouksey/go-init-setup/databa/go-init-setup/router"
	"github.com/sahilchouksey/go-init-setup/database"
)

func SetupAndRunServer() error {

	// Load ENV
	if err := config.LoadENV(); err != nil {
		return err

	}

	getEnv, err := config.Get()
	if err != nil {
		return err
	}

	// Initialize GORM database connection
	store, err := database.StartGORM()
	if err != nil {
		print("Check whether the Postgres is running or not\n")
		print("If not running, run the following command:\n")
		print("  make docker-up   (for Docker setup)\n")
		print("  make db-up       (for local PostgreSQL)\n")
		return err
	}

	if err := store.Init(); err != nil {
		print("Failed to initialize database tables\n")
		print("Error running migrations:\n")
		return err
	}

	// Initialize Cron Manager (only if enabled via environment variable)
	var cronManager *cron.CronManager
	if os.Getenv("CRON_ENABLED") != "false" { // Default to enabled
		db, ok := store.GetDB().(*gorm.DB)
		if !ok {
			print("Warning: Failed to get database connection for cron jobs\n")
		} else {
			cronManager = cron.NewCronManager(db)
			if err := cronManager.Start(); err != nil {
				print("Warning: Failed to start cron jobs\n")
				print("Error: ", err.Error(), "\n")
				// Don't fail the app, just log the warning
			}
		}
	}

	// Defer Closing DB and stopping cron jobs
	defer func() {
		if cronManager != nil {
			cronManager.Stop()
		}
		store.Close()
	}()

	// Init API
	var server *api.APIServer = api.NewAPIServer(fmt.Sprintf(":%d", getEnv.PORT))
	app := server.GetEngine()

	// Attach Middleware
	// Custom Logger
	app.Use(logger.New())

	app.Use(recover.New())

	// Setup Routes
	router.SetupRoutes(app, store)

	// Attach Swagger

	// Get the PORT & Start the Server
	return server.Run()

}
