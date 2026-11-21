// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package db

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// A real Hash160b[:8] collision cannot be constructed by hand, so the detection
// path is exercised by injecting a prefix function that collides on purpose.
func TestCheckNamespacePrefixCollisionDetects(t *testing.T) {
	r := require.New(t)
	// first byte only: "Account" and "Awful" collide, "Staking" does not
	firstByte := func(ns string) string {
		if len(ns) == 0 {
			return ""
		}
		return ns[:1]
	}
	err := checkNamespacePrefixCollision([]string{"Account", "Staking", "Awful"}, firstByte)
	r.Error(err)
	r.Contains(err.Error(), "Account")
	r.Contains(err.Error(), "Awful")
	r.Contains(err.Error(), "collision")

	r.NoError(checkNamespacePrefixCollision([]string{"Account", "Staking", "Rewarding"}, firstByte))
	// same namespace repeated is not a collision
	r.NoError(checkNamespacePrefixCollision([]string{"Account", "Account"}, firstByte))
	// real prefix function over the real namespaces
	r.NoError(CheckNamespacePrefixCollision([]string{"Account", "Staking", "Awful"}))
}
