package database

import (
	"os"
	"testing"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
	"github.com/sirupsen/logrus"
)

const deleteTablesGrace = time.Second * 5

var logger *logrus.Logger

func TestMain(m *testing.M) {
	logger = logrus.New()
	logger.SetLevel(logrus.TraceLevel)

	if err := initDB(true); err != nil {
		panic(err.Error())
	}

	os.Exit(m.Run())
}

func initDB(deleteDatabase bool) error {
	var dbConfig DatabaseConfig

	if err := cleanenv.ReadEnv(&dbConfig); err != nil {
		help, helpErr := cleanenv.GetDescription(&dbConfig, nil)
		if helpErr != nil {
			panic(helpErr.Error())
		}

		logger.Fatalf("Reading environment variables failed: %s\n%s", err.Error(), help)
	}

	if err := Init(logger, dbConfig); err != nil {
		return err
	}

	if !deleteDatabase {
		return nil
	}

	if err := deleteAllTables(); err != nil {
		return err
	}

	time.Sleep(deleteTablesGrace)

	// Prevents bug detection from triggering
	db.db = nil
	return initDB(false)
}
