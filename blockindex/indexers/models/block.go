package models

import "time"

type Block struct {
	BlockHeight     uint64    `gorm:"primary_key;" sql:"type:bigint"`
	BlockHash       string    `gorm:"size:64;not null;uniqueIndex"`
	ProducerAddress string    `gorm:"size:42;not null;index:,length:9"`
	NumActions      int       `gorm:"type:int2;unsigned;not null;default:0"`
	Timestamp       time.Time `gorm:"type:timestamp;index:,expression:(timestamp::date)"`
	Year            int       `gorm:"type:int2; unsigned;not null;default:0;index:idx_ymd"`
	Month           int       `gorm:"type:int2; unsigned;not null;default:0;index:idx_ymd"`
	Day             int       `gorm:"type:int2; unsigned;not null;default:0;index:idx_ymd"`
}

func (Block) TableName() string {
	return "block"
}
