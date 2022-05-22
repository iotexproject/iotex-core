// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package block

import (
	"context"
	"math/big"
	"strings"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestValidator(t *testing.T) {
	require := require.New(t)

	ctx := context.Background()
	blk := &Block{}

	v := NewValidator(nil)
	require.NoError(v.Validate(ctx, blk))

	valid := protocol.NewGenericValidator(nil, func(sr protocol.StateReader, addr address.Address) (*state.Account, error) {
		pk := identityset.PrivateKey(27).PublicKey()
		eAddr := pk.Address()
		if strings.EqualFold(eAddr.String(), addr.String()) {
			return nil, errors.New("MockChainManager nonce error")
		}
		account := state.NewEmptyAccount()
		require.NoError(account.SetNonce(2))
		return account, nil
	})

	tsf1, err := action.SignedTransfer(identityset.Address(28).String(), identityset.PrivateKey(27), 1, big.NewInt(20), []byte{}, 100000, big.NewInt(10))
	require.NoError(err)

	tsf2, err := action.SignedTransfer(identityset.Address(29).String(), identityset.PrivateKey(27), 1, big.NewInt(30), []byte{}, 100000, big.NewInt(10))
	require.NoError(err)

	blkhash, err := tsf1.Hash()
	require.NoError(err)
	nblk, err := NewTestingBuilder().
		SetHeight(1).
		SetPrevBlockHash(blkhash).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(tsf1, tsf2).
		SignAndBuild(identityset.PrivateKey(27))
	require.NoError(err)

	v = NewValidator(nil, valid)
	require.Contains(v.Validate(ctx, &nblk).Error(), "MockChainManager nonce error")

}
