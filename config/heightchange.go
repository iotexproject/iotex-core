// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package config

// Codename for height changes
const (
	Pacific = iota
	Aleutian
)

type (
	// HeightName is codename for height changes
	HeightName int

	// HeightChange lists heights at which certain fixes take effect
	HeightChange struct {
		pacificHeight  uint64
		aleutianHeight uint64
	}
)

// NewHeightChange creates a height change config
func NewHeightChange(cfg Config) HeightChange {
	return HeightChange{
		cfg.Genesis.PacificBlockHeight,
		cfg.Genesis.AleutianBlockHeight,
	}
}

// IsPost return true if height is after the height change
func (hc *HeightChange) IsPost(height uint64, name HeightName) bool {
	var h uint64
	if name == Pacific {
		h = hc.pacificHeight
	} else if name == Aleutian {
		h = hc.aleutianHeight
	} else {
		panic("invalid height name!")
	}
	return height >= h
}

// IsPre return true if height is before the height change
func (hc *HeightChange) IsPre(height uint64, name HeightName) bool {
	return !hc.IsPost(height, name)
}
