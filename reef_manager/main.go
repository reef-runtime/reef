package main

import (
	"fmt"
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/ilyakaznacheev/cleanenv"
	"github.com/reef-runtime/reef/reef_manager/api"
	"github.com/reef-runtime/reef/reef_manager/database"
	"github.com/sirupsen/logrus"
)

const LOG_LEVEL_DEFAULT = logrus.InfoLevel
const WEB_PORT = 3000

func main() {
	// Database connection.
	var dbConfig database.DatabaseConfig

	logger := newLogger()

	if err := cleanenv.ReadEnv(&dbConfig); err != nil {
		help, helpErr := cleanenv.GetDescription(&dbConfig, nil)
		if helpErr != nil {
			panic(helpErr.Error())
		}

		log.Fatalf("Reading environment variables failed: %s\n%s", err.Error(), help)
	}

	if err := database.Init(logger, dbConfig); err != nil {
		log.Fatalf("Initializing database failed: %s", err.Error())
	}

	// Fiber HTTP server.
	app := fiber.New()

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello, World!")
	})

	app.Get("/api/jobs", api.GetJobs)
	app.Post("/api/jobs/submit", api.SubmitJob)

	logger.Debugf("Starting web server on port %d...", WEB_PORT)
	log.Fatal(app.Listen(":" + fmt.Sprint(WEB_PORT)))
}
