// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package testutil

import (
	"math/rand"
	"time"
)

// RandomPort returns a random port number between 30000 and 50000
func RandomPort() int {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return r.Intn(20000) + 30000
}
