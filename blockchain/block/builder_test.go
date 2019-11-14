// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package block

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestBuilder(t *testing.T) {
	ra := NewRunnableActionsBuilder().Build()

	nblk, err := NewBuilder(ra).
		SetHeight(1).
		SetTimestamp(testutil.TimestampNow()).
		SetPrevBlockHash(hash.ZeroHash256).
		SignAndBuild(identityset.PrivateKey(29))
	require.NoError(t, err)

	require.True(t, nblk.VerifySignature())
}
