// Copyright (c) 2020 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package config

import (
	"log"

	"github.com/iotexproject/iotex-core/blockchain/genesis"
)

// Codename for height upgrades
const (
	Pacific = iota
	Aleutian
	Bering
	Cook
	Dardanelles
	Daytona
	Easter
	Fairbank
)

type (
	// HeightName is codename for height upgrades
	HeightName int

	// HeightUpgrade lists heights at which certain fixes take effect
	// prior to Dardanelles, each epoch consists of 360 sub-epochs
	// so height = 360k + 1
	// starting Dardanelles, each epoch consists of 720 sub-epochs
	// however, DardanellesHeight is set to 360(2k + 1) + 1 (instead of 720k + 1)
	// so height afterwards must be set to 360(2k + 1) + 1
	HeightUpgrade struct {
		pacificHeight     uint64
		aleutianHeight    uint64
		beringHeight      uint64
		cookHeight        uint64
		dardanellesHeight uint64
		daytonaHeight     uint64
		easterHeight      uint64
		fairbankHeight    uint64
	}
)

// NewHeightUpgrade creates a height upgrade config
func NewHeightUpgrade(cfg *genesis.Genesis) HeightUpgrade {
	return HeightUpgrade{
		cfg.PacificBlockHeight,
		cfg.AleutianBlockHeight,
		cfg.BeringBlockHeight,
		cfg.CookBlockHeight,
		cfg.DardanellesBlockHeight,
		cfg.DaytonaBlockHeight,
		cfg.EasterBlockHeight,
		cfg.FairbankBlockHeight,
	}
}

// IsPost return true if height is after the height upgrade
func (hu *HeightUpgrade) IsPost(name HeightName, height uint64) bool {
	var h uint64
	switch name {
	case Pacific:
		h = hu.pacificHeight
	case Aleutian:
		h = hu.aleutianHeight
	case Bering:
		h = hu.beringHeight
	case Cook:
		h = hu.cookHeight
	case Dardanelles:
		h = hu.dardanellesHeight
	case Daytona:
		h = hu.daytonaHeight
	case Easter:
		h = hu.easterHeight
	case Fairbank:
		h = hu.fairbankHeight
	default:
		log.Panic("invalid height name!")
	}
	return height >= h
}

// IsPre return true if height is before the height upgrade
func (hu *HeightUpgrade) IsPre(name HeightName, height uint64) bool {
	return !hu.IsPost(name, height)
}

// PacificBlockHeight returns the pacific height
func (hu *HeightUpgrade) PacificBlockHeight() uint64 { return hu.pacificHeight }

// AleutianBlockHeight returns the aleutian height
func (hu *HeightUpgrade) AleutianBlockHeight() uint64 { return hu.aleutianHeight }

// BeringBlockHeight returns the bering height
func (hu *HeightUpgrade) BeringBlockHeight() uint64 { return hu.beringHeight }

// CookBlockHeight returns the cook height
func (hu *HeightUpgrade) CookBlockHeight() uint64 { return hu.cookHeight }

// DardanellesBlockHeight returns the dardanelles height
func (hu *HeightUpgrade) DardanellesBlockHeight() uint64 { return hu.dardanellesHeight }

// DaytonaBlockHeight returns the daytona height
func (hu *HeightUpgrade) DaytonaBlockHeight() uint64 { return hu.daytonaHeight }

// EasterBlockHeight returns the easter height
func (hu *HeightUpgrade) EasterBlockHeight() uint64 { return hu.easterHeight }

// FairbankBlockHeight returns the fairbank height
func (hu *HeightUpgrade) FairbankBlockHeight() uint64 { return hu.fairbankHeight }
