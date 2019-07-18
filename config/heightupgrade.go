// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package config

import (
	"log"
)

// Codename for height upgrades
const (
	Pacific = iota
	Aleutian
)

type (
	// HeightName is codename for height upgrades
	HeightName int

	// HeightUpgrade lists heights at which certain fixes take effect
	HeightUpgrade struct {
		pacificHeight  uint64
		aleutianHeight uint64
	}
)

// NewHeightUpgrade creates a height upgrade config
func NewHeightUpgrade(cfg Config) HeightUpgrade {
	return HeightUpgrade{
		cfg.Genesis.PacificBlockHeight,
		cfg.Genesis.AleutianBlockHeight,
	}
}

// IsPost return true if height is after the height upgrade
func (hu *HeightUpgrade) IsPost(height uint64, name HeightName) bool {
	var h uint64
	if name == Pacific {
		h = hu.pacificHeight
	} else if name == Aleutian {
		h = hu.aleutianHeight
	} else {
		log.Panic("invalid height name!")
	}
	return height >= h
}

// IsPre return true if height is before the height upgrade
func (hu *HeightUpgrade) IsPre(height uint64, name HeightName) bool {
	return !hu.IsPost(height, name)
}
