package db

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/pkg/errors"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/iotexproject/iotex-core/db/batch"
	"github.com/iotexproject/iotex-core/pkg/lifecycle"
)

type PostgresDB struct {
	lifecycle.Readiness
	db     *gorm.DB
	config Config
}

// TODO kv store or rmdb?

// NewPostgresDB instantiates a PostgresDB with implements KVStore
func NewPostgresDB(cfg Config) *PostgresDB {
	return &PostgresDB{
		Readiness: lifecycle.Readiness{},
		config:    cfg,
	}
}

func (p *PostgresDB) DB() *gorm.DB {
	return p.db
}

func (p *PostgresDB) Start(_ context.Context) error {
	if p.IsReady() {
		return nil
	}

	newLogger := logger.Discard
	if p.config.DbDebug {
		newLogger = logger.New(
			log.New(os.Stdout, "\r\n", log.LstdFlags), // io writer
			logger.Config{
				SlowThreshold:             time.Second,   // Slow SQL threshold
				LogLevel:                  logger.Silent, // Log level
				IgnoreRecordNotFoundError: true,          // Ignore ErrRecordNotFound error for logger
				Colorful:                  false,         // Disable color
			},
		)
	}
	gormConfig := &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		SkipDefaultTransaction:                   false,
		PrepareStmt:                              true,
		Logger:                                   newLogger,
	}

	db, err := gorm.Open(postgres.Open(p.config.DbDSN), gormConfig)
	if err != nil {
		return errors.Wrap(err, "failed to connect analyser's postgres")
	}

	p.db = db
	if p.config.DbDebug {
		p.db = db.Debug()
	}
	return p.TurnOn()
}

func (p *PostgresDB) Stop(_ context.Context) error {
	if err := p.TurnOff(); err != nil {
		return err
	}
	dbInstance, err := p.db.DB()
	if err != nil {
		return err
	}
	if err := dbInstance.Close(); err != nil {
		return err
	}
	return nil
}

func (p *PostgresDB) Put(namespace string, key, value []byte) (err error) {
	return nil
}

func (p *PostgresDB) Get(namespace string, key []byte) ([]byte, error) {
	return nil, nil
}

func (p *PostgresDB) Delete(namespace string, key []byte) (err error) {
	return nil
}

func (p *PostgresDB) WriteBatch(kvsb batch.KVStoreBatch) (err error) {
	if !p.IsReady() {
		return ErrDBNotStarted
	}

	return
}

func (p *PostgresDB) Filter(namespace string, cond Condition, minKey, maxKey []byte) ([][]byte, [][]byte, error) {
	return nil, nil, nil
}
