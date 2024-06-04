package database

import (
	"database/sql"
	"errors"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"

	// Required for DB migrations.
	_ "github.com/golang-migrate/migrate/v4/source/pkger"
	// Also required for DB migrations.
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

var db = Database{
	builder: sq.StatementBuilderType{},
	db:      nil,
}

//
// Database initialization.
//

type DBConfig struct {
	Username string `env:"REEF_DB_USERNAME" env-required:"true"`
	Password string `env:"REEF_DB_PASSWORD" env-required:"true"`
	Host     string `env:"REEF_DB_HOST"     env-required:"true"`
	Port     uint16 `env:"REEF_DB_PORT"     env-required:"true"`
	DBName   string `env:"REEF_DB_NAME"     env-required:"true"`
}

func Init(pLogger *logrus.Logger, config DBConfig) error {
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

	// nolint:goconst
	dbTemp, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Errorf("Could not connect to database: %s", err.Error())
		return err
	}

	// Run migrations.
	// nolint:exhaustruct
	migratorConfig := postgres.Config{}

	driver, err := postgres.WithInstance(dbTemp, &migratorConfig)
	if err != nil {
		log.Errorf("Could not run migrations: failed to create migration instance %s", err.Error())
		return err
	}

	m, err := migrate.NewWithDatabaseInstance(
		"pkger:///db/migrations",
		"postgres", driver)
	if err != nil {
		log.Errorf("Could not run migrations: failed to connect to database: %s", err.Error())
		return err
	}

	err = m.Up()
	if errors.Is(err, migrate.ErrNoChange) {
		log.Trace("Database migrations were not executed, no need for changes.")
	} else if err != nil {
		log.Errorf("Could not run migrations: failed to run UP migrations: %s", err.Error())
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
