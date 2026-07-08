// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package actpool

import (
	"context"
	"math/big"
	"testing"

	"github.com/iotexproject/iotex-address/address"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_chainmanager"
)

func TestActPool_GetUnconfirmedActsEdge(t *testing.T) {
	r := require.New(t)
	ap, _ := newTestWorkerActPool(t)

	// malformed address -> empty slice, no panic
	r.Empty(ap.GetUnconfirmedActs("not-a-valid-address"))
	// well-formed but unknown account -> empty slice
	r.Empty(ap.GetUnconfirmedActs(_addr1))
}

// TestActPool_DeleteAction goes through the real Add path so that every coupled
// index (allActions, gasInPool, destination map) is populated, then asserts that
// DeleteAction unwinds all of them together.
func TestActPool_DeleteAction(t *testing.T) {
	ctrl := gomock.NewController(t)
	r := require.New(t)
	sf := mock_chainmanager.NewMockStateReader(ctrl)
	Ap, err := NewActPool(genesis.TestDefault(), sf, getActPoolCfg())
	r.NoError(err)
	ap := Ap.(*actPool)
	ap.AddActionEnvelopeValidators(protocol.NewGenericValidator(sf, accountutil.AccountState))

	// nil caller is a no-op and must not panic
	ap.DeleteAction(nil)

	// _addr1 (priKey1) sends two transfers to _addr2
	tsf1, err := action.SignedTransfer(_addr2, _priKey1, uint64(1), big.NewInt(10), []byte{}, uint64(100000), big.NewInt(0))
	r.NoError(err)
	tsf2, err := action.SignedTransfer(_addr2, _priKey1, uint64(2), big.NewInt(20), []byte{}, uint64(100000), big.NewInt(0))
	r.NoError(err)
	sf.EXPECT().State(gomock.Any(), gomock.Any()).DoAndReturn(func(acct interface{}, opts ...protocol.StateOption) (uint64, error) {
		a := acct.(*state.Account)
		r.NoError(a.AddBalance(big.NewInt(100000000000000000)))
		return 0, nil
	}).AnyTimes()
	sf.EXPECT().Height().Return(uint64(1), nil).AnyTimes()
	ctx := genesis.WithGenesisContext(context.Background(), genesis.TestDefault())
	r.NoError(ap.Add(ctx, tsf1))
	r.NoError(ap.Add(ctx, tsf2))

	// pool now holds two actions, non-zero gas, and a destination index for _addr2
	r.Equal(uint64(2), ap.GetSize())
	r.Greater(ap.GetGasSize(), uint64(0))
	r.Len(ap.GetUnconfirmedActs(_addr2), 2)

	addr, err := address.FromString(_addr1)
	r.NoError(err)
	ap.DeleteAction(addr)

	// everything the two actions touched must be unwound
	r.Equal(uint64(0), ap.GetSize())
	r.Equal(uint64(0), ap.GetGasSize()) // gas counter must not underflow
	r.Nil(ap.worker[ap.allocatedWorker(addr)].accountActs.Account(_addr1))
	r.Empty(ap.GetUnconfirmedActs(_addr2)) // destination map cleaned up
}
