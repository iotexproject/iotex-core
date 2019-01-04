package sqlite3

import (
	"context"
	"log"
	"sync"

	"database/sql"
	_ "github.com/mattn/go-sqlite3"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/lifecycle"
)

// Store is the interface of KV store.
type Store interface {
	lifecycle.StartStopper

	// Get DB instance
	GetDB() *sql.DB

	// Transact wrap the transaction
	Transact(txFunc func(*sql.Tx) error) error
}

// sqlite3 is local sqlite3
type sqlite3 struct {
	mutex  sync.RWMutex
	db     *sql.DB
	config *config.SQLITE3
}

// NewSQLite3 instantiates an sqlite3
func NewSQLite3(cfg *config.SQLITE3) Store {
	return &sqlite3{db: nil, config: cfg}
}

// Start opens the RDS (creates new file if not existing yet)
func (s *sqlite3) Start(_ context.Context) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.db != nil {
		return nil
	}

	// Use db to perform SQL operations on database
	db, err := sql.Open("sqlite3", s.config.SQLite3File)
	if err != nil {
		return err
	}
	s.db = db
	return nil
}

// Stop closes the AWS RDS
func (s *sqlite3) Stop(_ context.Context) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.db != nil {
		err := s.db.Close()
		s.db = nil
		return err
	}
	return nil
}

// Stop closes the AWS RDS
func (s *sqlite3) GetDB() *sql.DB {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.db
}

// Transact wrap the transaction
func (s *sqlite3) Transact(txFunc func(*sql.Tx) error) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if p := recover(); p != nil {
			err = tx.Rollback()
			if err != nil {
				log.Fatal(err) // log err after Rollback
			}
		} else if err != nil {
			err = tx.Rollback() // err is non-nil; don't change it
			if err != nil {
				log.Fatal(err)
			}
		} else {
			err = tx.Commit() // err is nil; if Commit returns error update err
			if err != nil {
				log.Fatal(err)
			}
		}
	}()
	err = txFunc(tx)
	return err
}
