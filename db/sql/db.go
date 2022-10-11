package sql

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
)

var db *sql.DB
var dbOpened bool

// Database is the configuration of database
type Database struct {
	Host     string `yaml:"host"`
	Port     string `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Name     string `yaml:"name"`
}

// DSN returns the data source name
func (cfg *Database) DSN() string {
	return fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable", cfg.Host, cfg.User, cfg.Password, cfg.Name, cfg.Port)
}

// Open opens the database
func Open(cfg *Database) (*sql.DB, error) {
	var err error
	db, err = sql.Open("postgres", cfg.DSN())
	if err == nil {
		dbOpened = true
	}
	return db, err
}
