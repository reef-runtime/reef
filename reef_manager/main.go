package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/ilyakaznacheev/cleanenv"
	"github.com/reef-runtime/reef/reef_manager/api"
	"github.com/reef-runtime/reef/reef_manager/database"
	"github.com/reef-runtime/reef/reef_manager/logic"
	"github.com/sirupsen/logrus"
)

const webPort = 3000
const datasetPath = "./datasets/"

type Config struct {
	Database       database.DBConfig
	CompilerConfig logic.CompilerConfig
}

//nolint:funlen
func ship(logger *logrus.Logger) error {
	//
	// Database connection.
	//
	var config Config

	if err := cleanenv.ReadEnv(&config); err != nil {
		help, helpErr := cleanenv.GetDescription(&config, nil)
		if helpErr != nil {
			return err
		}

		return fmt.Errorf("configuration error: %s", help)
	}

	if err := database.Init(logger, config.Database); err != nil {
		logger.Fatalf("Initializing database failed: %s", err.Error())
		return errors.New("database error")
	}

	if err := logic.Init(logger, datasetPath); err != nil {
		logger.Fatalf("Initializing logic package failed: %s", err.Error())
		return errors.New("system error")
	}

	//
	// HTTP server.
	//
	r := gin.Default()

	r.GET("/", func(ctx *gin.Context) {
		ctx.String(http.StatusOK, "REEF")
	})

	//
	// Jobs.
	//
	r.GET("/api/jobs", api.GetJobs)
	r.GET("/api/result", api.GetResult)
	r.POST("/api/jobs/submit", api.SubmitJob)
	r.DELETE("/api/jobs/abort", api.AbortJob)

	//
	// Nodes.
	//
	r.GET("/api/node/connect", api.HandleNodeConnection)

	//
	// Datasets.
	//
	r.GET("/api/datasets", api.GetDatasets)
	r.POST("/api/datasets/upload", api.UploadDataset)
	r.DELETE("/api/datasets/delete", api.DeleteDataset)

	//
	// Logs.
	//
	r.GET("/api/logs", api.GetLogs)

	logger.Debugf("Starting web server on port %d...", webPort)

	go logic.JobManager.JobQueueDaemon()

	logic.InitCompiler(config.CompilerConfig)

	api.Init(logger)
	if err := r.Run(":" + fmt.Sprint(webPort)); err != nil {
		return fmt.Errorf("failed to run webserver: %s", err.Error())
	}

	return nil
}

func main() {
	logger := newLogger()

	if err := ship(logger); err != nil {
		logger.Errorf("Failed to start sailing: %s", err.Error())
		os.Exit(1)
	}
}
