package rds

import (
	"context"
	"fmt"
	"log"
	"sync"

	"database/sql"

	// we need mysql import because it's called in file, (but compile will complain because there is no display)
	_ "github.com/go-sql-driver/mysql"
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

// rds is RDSStore implementation based aws RDS
type rds struct {
	mutex  sync.RWMutex
	db     *sql.DB
	config *config.RDS
}

// NewAwsRDS instantiates an aws RDS based RDS
func NewAwsRDS(cfg *config.RDS) Store {
	return &rds{db: nil, config: cfg}
}

// Start opens the RDS (creates new file if not existing yet)
func (r *rds) Start(_ context.Context) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.db != nil {
		return nil
	}

	connectStr := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s",
		r.config.AwsRDSUser, r.config.AwsPass, r.config.AwsRDSEndpoint, r.config.AwsRDSPort, r.config.AwsDBName,
	)

	// Use db to perform SQL operations on database
	db, err := sql.Open("mysql", connectStr)
	if err != nil {
		return err
	}
	r.db = db
	return nil
}

// Stop closes the AWS RDS
func (r *rds) Stop(_ context.Context) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.db != nil {
		err := r.db.Close()
		r.db = nil
		return err
	}
	return nil
}

// Stop closes the AWS RDS
func (r *rds) GetDB() *sql.DB {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	return r.db
}

// Transact wrap the transaction
func (r *rds) Transact(txFunc func(*sql.Tx) error) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if p := recover(); p != nil {
			err = tx.Rollback()
			log.Fatal(err) // log err after Rollback
		} else if err != nil {
			err = tx.Rollback() // err is non-nil; don't change it
			log.Fatal(err)
		} else {
			err = tx.Commit() // err is nil; if Commit returns error update err
			log.Fatal(err)
		}
	}()
	err = txFunc(tx)
	return err
}
