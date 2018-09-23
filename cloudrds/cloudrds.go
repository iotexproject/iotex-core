package cloudrds

import (
	"context"
	"database/sql"
	"fmt"
	"sync"

	// we need mysql import because it's called in file, (but compile will complain because there is no display)
	_ "github.com/go-sql-driver/mysql"
	"github.com/iotexproject/iotex-core/pkg/lifecycle"
)

var (
	awsRDSEndpoint = "iotex-explorer-db.ctcedgqcwrb5.us-west-1.rds.amazonaws.com"
	awsRDSPort     = 4086
	awsRegion      = "us-west-1"
	awsRDSUser     = "explorer_admin"
	awsPass        = "j1cDiH7W7QCB"
	awsDBName      = "explorer"
)

// RDSStore is the interface of KV store.
type RDSStore interface {
	lifecycle.StartStopper

	// Get DB instance
	GetDB() *sql.DB

	// ParseRows parse the returned raw rows
	ParseRows(rows *sql.Rows) ([][]sql.RawBytes, error)

	// Transact wrap the transaction
	Transact(txFunc func(*sql.Tx) error) error
}

// rds is RDSStore implementation based aws RDS
type rds struct {
	mutex sync.RWMutex
	db    *sql.DB
}

// NewAwsRDS instantiates an aws RDS based RDS
func NewAwsRDS() RDSStore {
	return &rds{db: nil}
}

// Start opens the RDS (creates new file if not existing yet)
func (r *rds) Start(_ context.Context) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.db != nil {
		return nil
	}

	connectStr := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s",
		awsRDSUser, awsPass, awsRDSEndpoint, awsRDSPort, awsDBName,
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
	return r.db
}

// Parse rows
func (r *rds) ParseRows(rows *sql.Rows) ([][]sql.RawBytes, error) {
	var parsedRows [][]sql.RawBytes

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	fmt.Println(columns)

	// Make a slice for the values
	values := make([]sql.RawBytes, len(columns))
	fmt.Println(values)

	// rows.Scan wants '[]interface{}' as an argument, so we must copy the
	// references into such a slice
	// See http://code.google.com/p/go-wiki/wiki/InterfaceSlice for details
	scanArgs := make([]interface{}, len(values))
	for i := range values {
		scanArgs[i] = &values[i]
	}
	fmt.Println(scanArgs)

	// Fetch rows
	for rows.Next() {
		// get RawBytes from data
		err = rows.Scan(scanArgs...)
		if err != nil {
			panic(err.Error()) // proper error handling instead of panic in your app
		}

		parsedRows = append(parsedRows, values)
	}

	return parsedRows, nil
}

// Transact wrap the transaction
func (r *rds) Transact(txFunc func(*sql.Tx) error) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p) // re-throw panic after Rollback
		} else if err != nil {
			tx.Rollback() // err is non-nil; don't change it
		} else {
			err = tx.Commit() // err is nil; if Commit returns error update err
		}
	}()
	err = txFunc(tx)
	return err
}
