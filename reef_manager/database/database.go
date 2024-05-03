package database

import (
	"database/sql"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	_ "github.com/lib/pq"
	"github.com/sirupsen/logrus"
)

var log *logrus.Logger

func initLogger(logger *logrus.Logger) {
	log = logger
}

//
// Database connection.
//

type Database struct {
	builder sq.StatementBuilderType
	db      *sql.DB
}

var db Database = Database{
	builder: sq.StatementBuilderType{},
	db:      nil,
}

//
// Database initialization.
//

type DatabaseConfig struct {
	Username string `env:"REEF_DB_USERNAME" env-required:"true"`
	Password string `env:"REEF_DB_PASSWORD" env-required:"true"`
	Host     string `env:"REEF_DB_HOST" env-required:"true"`
	Port     uint16 `env:"REEF_DB_PORT" env-required:"true"`
	DBName   string `env:"REEF_DB_NAME" env-required:"true"`
}

func Init(pLogger *logrus.Logger, config DatabaseConfig) error {
	initLogger(pLogger)

	if db.db != nil {
		panic("[BUG] Database connection is already initialized")
	}

	log.Debug("Initializing database connection...")

	connStr := fmt.Sprintf(
		"postgresql://%s:%s@%s:%d/%s?sslmode=disable",
		config.Username,
		config.Password,
		config.Host,
		config.Port,
		config.DBName,
	)

	dbTemp, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Errorf("Could not connect to database: %s", err.Error())
		return err
	}

	dbCache := sq.NewStmtCache(dbTemp)
	builder := sq.StatementBuilder.PlaceholderFormat(sq.Dollar).RunWith(dbCache)

	db = Database{
		builder: builder,
		db:      dbTemp,
	}

	return nil
}

// Solely used for testing purposes.
func deleteAllTables() error {
	tables := []string{JobTableName}

	for _, table := range tables {
		if _, err := db.builder.Delete(table).Exec(); err != nil {
			return err
		}
	}

	return nil
}
