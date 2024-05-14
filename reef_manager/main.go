package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ilyakaznacheev/cleanenv"
	"github.com/reef-runtime/reef/reef_manager/api"
	"github.com/reef-runtime/reef/reef_manager/database"
	"github.com/reef-runtime/reef/reef_manager/logic"
)

const WEB_PORT = 3000
const DATASET_PATH = "./datasets/"

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

	if err := logic.Init(logger, DATASET_PATH); err != nil {
		log.Fatalf("Initializing logic package failed: %s", err.Error())
	}

	// HTTP server.
	r := gin.Default()

	r.GET("/", func(ctx *gin.Context) {
		ctx.String(http.StatusOK, "REEF")
	})

	// Jobs.
	r.GET("/api/jobs", api.GetJobs)
	r.POST("/api/jobs/submit", api.SubmitJob)
	r.DELETE("/api/jobs/abort", api.AbortJob)

	// Nodes.
	r.GET("/api/node/connect", api.HandleNodeConnection)

	// Datasets.
	r.GET("/api/datasets", api.GetDatasets)
	r.POST("/api/datasets/upload", api.UploadDataset)
	r.DELETE("/api/datasets/delete", api.DeleteDataset)

	logger.Debugf("Starting web server on port %d...", WEB_PORT)

	api.Init(logger)
	log.Fatal(r.Run(":" + fmt.Sprint(WEB_PORT)))
}
