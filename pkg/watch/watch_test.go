// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package watch

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/testutil"
)

func TestStart(t *testing.T) {
	require := require.New(t)
	h := Start(context.Background(), 30*time.Millisecond)
	require.NoError(testutil.WaitUntil(100*time.Millisecond, 1*time.Second, func() (b bool, e error) {
		h()
		return true, nil
	}))
}
