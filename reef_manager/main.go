package main

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/ilyakaznacheev/cleanenv"
	"github.com/reef-runtime/reef/reef_manager/api"
	"github.com/reef-runtime/reef/reef_manager/database"
	"github.com/reef-runtime/reef/reef_manager/logic"
	"github.com/sirupsen/logrus"
)

//
// Configuration via environment variables.
//

type Config struct {
	DatasetPath       string `env:"REEF_DATASETS_PATH"        env-required:"true"`
	Port              uint16 `env:"REEF_MANAGER_PORT"         env-required:"true"`
	TemplatesDirPath  string `env:"REEF_TEMPLATES_PATH"       env-required:"true"`
	AdminToken        string `env:"REEF_ADMIN_TOKEN"          env-required:"true"`
	SessionSecret     string `env:"REEF_SESSION_SECRET"       env-required:"true"`
	MaxJobRuntime     uint64 `env:"REEF_JOB_MAX_RUNTIME_SECS" env-required:"true"`
	NodeNameBlacklist string `env:"REEF_NODES_BLACKLIST"      env-required:"true"`
	Database          database.DatabaseConfig
	CompilerConfig    logic.CompilerConfig
}

//go:embed database/migrations/*.sql
var migrations embed.FS

const embedPath = "database/migrations"

//
// HTTP server.
//

func httpServe(logger *logrus.Logger, config *Config) error {
	r := gin.Default()

	//
	// Authentication.
	//
	// This is only required so that user sessions are signed securely, use a randome value here.
	store := cookie.NewStore([]byte(config.SessionSecret))
	r.Use(sessions.Sessions(api.SessionName, store))

	api.InitAuthHandler(config.AdminToken)

	r.POST("/api/auth", api.AuthHandler.HandleAuth)

	// Require authentication for the entire API.
	apiGroup := r.Group("/api")
	apiGroup.Use(api.AuthHandler.ReefAuth())

	//
	// Jobs.
	//
	apiGroup.GET("/templates", api.GetTemplates)
	apiGroup.GET("/jobs", api.GetJobs)
	apiGroup.GET("/job/:job_id", api.GetJob)
	apiGroup.GET("/job/result/:job_id", api.GetResult)
	apiGroup.POST("/jobs/submit", api.SubmitJob)
	apiGroup.DELETE("/job/abort", api.AbortOrCancelJob)

	//
	// Nodes.
	//
	r.GET("/api/node/connect", api.HandleNodeConnection)
	apiGroup.GET("/nodes", api.GetNodes)

	//
	// Datasets.
	//
	apiGroup.GET("/datasets", api.GetDatasets)
	apiGroup.POST("/datasets/upload", api.UploadDataset)
	apiGroup.DELETE("/datasets/delete", api.DeleteDataset)
	r.GET("/api/dataset/:id", api.LoadDataset)

	//
	// Logs.
	//
	apiGroup.GET("/logs", api.GetLogs)

	//
	// UI websocket with notifications.
	//
	apiGroup.GET("/updates", logic.UIManager.InitConn)

	logger.Debugf("Starting web server on port %d...", config.Port)

	api.Init(logger)
	if err := r.Run(":" + fmt.Sprint(config.Port)); err != nil {
		return fmt.Errorf("failed to run webserver: %s", err.Error())
	}

	return nil
}

func sail(logger *logrus.Logger) error {
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

	//
	// Parse env variables further.
	//

	nodesBlackList := make([]string, 0)
	if err := json.Unmarshal([]byte(config.NodeNameBlacklist), &nodesBlackList); err != nil {
		return fmt.Errorf("could not parse node blacklist: invalid JSON: %s", err.Error())
	}

	if err := database.Init(logger, config.Database, migrations, embedPath); err != nil {
		logger.Fatalf("Initializing database failed: %s", err.Error())
		return errors.New("database error")
	}

	//
	// Logic layer initialization.
	//
	if err := logic.Init(
		logger,
		config.CompilerConfig,
		config.DatasetPath,
		config.TemplatesDirPath,
		config.MaxJobRuntime,
		nodesBlackList,
	); err != nil {
		logger.Fatalf("Initializing logic package failed: %s", err.Error())
		return errors.New("system error")
	}

	return httpServe(logger, &config)
}

func main() {
	logger := newLogger()

	if err := sail(logger); err != nil {
		logger.Errorf("Failed to start sailing: %s", err.Error())
		os.Exit(1)
	}
}
