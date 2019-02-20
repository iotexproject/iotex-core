// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package block

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/pkg/hash"
	ta "github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestTestingBuilder(t *testing.T) {
	nblk, err := NewTestingBuilder().
		SetHeight(1).
		SetPrevBlockHash(hash.ZeroHash256).
		SetTimeStamp(testutil.TimestampNow()).
		SignAndBuild(ta.Keyinfo["bravo"].PubKey, ta.Keyinfo["bravo"].PriKey)
	require.NoError(t, err)

	require.True(t, nblk.VerifySignature())
}
