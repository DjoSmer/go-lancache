package database

import (
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"go-lancache/internal/config"
)

func New(cfg config.PostgresConfig, log *slog.Logger) *sql.DB {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DB)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		panic(err)
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(10 * time.Minute)

	err = db.Ping()
	if err != nil {
		panic(err)
	}

	log.Debug("Successfully connected!")

	return db
}
