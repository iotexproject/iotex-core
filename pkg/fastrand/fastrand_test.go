// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package fastrand

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRead(t *testing.T) {
	for _, test := range []int{16, 17} {
		p := make([]byte, test)
		n := Read(p)
		require.Equal(t, test, n)
	}
}
