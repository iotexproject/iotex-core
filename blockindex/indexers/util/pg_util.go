package util

import (
	"sync"

	"gorm.io/gorm"

	"github.com/iotexproject/iotex-core/db"
)

type IndexHeight struct {
	ID     uint32 `gorm:"primary_key;auto_increment"`
	Name   string `gorm:"size:128;not null;unique"`
	Height uint64 `sql:"type:bigint"`
}

var indexCache sync.Map

func Init(p *db.PostgresDB) error {
	return p.DB().AutoMigrate(&IndexHeight{})
}

func UpdateIndexHeightByTx(tx *gorm.DB, name string, height uint64) error {
	indexCache.Store(name, height)
	return tx.Model(&IndexHeight{}).Where("name = ?", name).UpdateColumn("height", height).Error
}

func GetIndexHeight(p *db.PostgresDB, name string) (uint64, error) {
	height, ok := indexCache.Load(name)
	if ok {
		return height.(uint64), nil
	}
	var m *IndexHeight
	idx, err := m.ByName(p, name)
	if err != nil {
		if err != gorm.ErrRecordNotFound {
			return 0, err
		}
		m = &IndexHeight{
			Name: name,
		}
		return 0, p.DB().Create(m).Error
	}
	indexCache.Store(name, idx.Height)
	return idx.Height, nil
}

func (m *IndexHeight) ByName(p *db.PostgresDB, name string) (*IndexHeight, error) {
	var err error
	err = p.DB().Model(m).Where("name = ?", name).Take(&m).Error
	if err != nil {
		return nil, err
	}
	return m, err
}

// AutoMigrate run auto migration for given models
func AutoMigrate(p *db.PostgresDB, index string, dst ...interface{}) error {
	height, err := GetIndexHeight(p, index)
	if err != nil {
		return err
	}
	if height == 0 {
		err = p.DB().Migrator().DropTable(dst...)
		if err != nil {
			return err
		}
		return p.DB().Migrator().CreateTable(dst...)
	}
	return nil
}
