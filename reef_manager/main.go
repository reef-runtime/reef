package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ilyakaznacheev/cleanenv"
	"github.com/reef-runtime/reef/reef_manager/api"
	"github.com/reef-runtime/reef/reef_manager/database"
)

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
	r := gin.Default()

	r.GET("/", func(ctx *gin.Context) {
		ctx.String(http.StatusOK, "REEF")
	})

	r.GET("/api/jobs", api.GetJobs)
	r.POST("/api/jobs/submit", api.SubmitJob)
	r.DELETE("/api/jobs/abort", api.AbortJob)

	logger.Debugf("Starting web server on port %d...", WEB_PORT)
	log.Fatal(r.Run(":" + fmt.Sprint(WEB_PORT)))
}
