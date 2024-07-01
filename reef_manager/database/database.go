package database

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/golang-migrate/migrate/v4/source/iofs"
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

func Init(pLogger *logrus.Logger, config DatabaseConfig, migrations embed.FS) error {
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

	// Run migrations.
	MIGRATOR_CONFIG := postgres.Config{}

	driver, err := postgres.WithInstance(dbTemp, &MIGRATOR_CONFIG)
	if err != nil {
		log.Errorf("Could not run migrations: failed to create migration instance %s", err.Error())
		return err
	}

	source, err := iofs.New(migrations, "db/migrations")
	if err != nil {
		return fmt.Errorf("migration ebmed failed: %s", err.Error())
	}

	m, err := migrate.NewWithInstance(
		"iofs",
		source,
		"postgres",
		driver,
	)
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

// // Solely used for testing purposes.
// func deleteAllTables() error {
// 	tables := []string{JobTableName}

// 	for _, table := range tables {
// 		if _, err := db.builder.Delete(table).Exec(); err != nil {
// 			return err
// 		}
// 	}

// 	return nil
// }
