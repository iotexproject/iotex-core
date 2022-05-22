// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"context"
	"math/big"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil"
	"github.com/iotexproject/iotex-core/testutil/testdb"
)

func TestProtocol_ValidateTransfer(t *testing.T) {
	require := require.New(t)
	p := NewProtocol(rewarding.DepositGas)
	t.Run("invalid transfer", func(t *testing.T) {
		tsf, err := action.NewTransfer(uint64(1), big.NewInt(1), "2", make([]byte, 32683), uint64(0), big.NewInt(0))
		require.NoError(err)
		tsf1, err := action.NewTransfer(uint64(1), big.NewInt(1), "2", nil, uint64(0), big.NewInt(0))
		require.NoError(err)
		g := genesis.Default
		ctx := protocol.WithFeatureCtx(genesis.WithGenesisContext(protocol.WithBlockCtx(context.Background(), protocol.BlockCtx{
			BlockHeight: g.NewfoundlandBlockHeight,
		}), g))
		for _, v := range []struct {
			tsf *action.Transfer
			err error
		}{
			{tsf, action.ErrOversizedData},
			{tsf1, address.ErrInvalidAddr},
		} {
			require.Equal(v.err, errors.Cause(p.Validate(ctx, v.tsf, nil)))
		}
	})
}

func TestProtocol_HandleTransfer(t *testing.T) {
	require := require.New(t)

	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)

	// set-up protocol and genesis states
	p := NewProtocol(rewarding.DepositGas)
	reward := rewarding.NewProtocol(config.Default.Genesis.Rewarding)
	registry := protocol.NewRegistry()
	require.NoError(reward.Register(registry))
	chainCtx := genesis.WithGenesisContext(
		protocol.WithRegistry(context.Background(), registry),
		config.Default.Genesis,
	)
	ctx := protocol.WithBlockCtx(chainCtx, protocol.BlockCtx{})
	ctx = protocol.WithFeatureCtx(ctx)
	require.NoError(reward.CreateGenesisStates(ctx, sm))

	// initial deposit to alfa and charlie (as a contract)
	alfa := identityset.Address(28)
	bravo := identityset.Address(29)
	charlie := identityset.Address(30)
	acct1 := state.NewEmptyAccount()
	require.NoError(acct1.AddBalance(big.NewInt(50005)))
	require.NoError(accountutil.StoreAccount(sm, alfa, acct1))
	acct2 := state.NewEmptyAccount()
	acct2.CodeHash = []byte("codeHash")
	require.NoError(accountutil.StoreAccount(sm, charlie, acct2))

	tests := []struct {
		caller      address.Address
		nonce       uint64
		amount      *big.Int
		recipient   string
		gasLimit    uint64
		gasPrice    *big.Int
		isContract  bool
		err         error
		status      uint64
		contractLog uint64
	}{
		{
			alfa, 1, big.NewInt(2), bravo.String(), 10000, big.NewInt(1), false, nil, uint64(iotextypes.ReceiptStatus_Success), 2,
		},
		// transfer to contract address only charges gas fee
		{
			alfa, 2, big.NewInt(20), charlie.String(), 10000, big.NewInt(1), true, nil, uint64(iotextypes.ReceiptStatus_Failure), 1,
		},
		// not enough balance
		{
			alfa, 3, big.NewInt(30000), bravo.String(), 10000, big.NewInt(1), false, state.ErrNotEnoughBalance, uint64(iotextypes.ReceiptStatus_Failure), 1,
		},
	}

	for _, v := range tests {
		tsf, err := action.NewTransfer(v.nonce, v.amount, v.recipient, []byte{}, v.gasLimit, v.gasPrice)
		require.NoError(err)
		gas, err := tsf.IntrinsicGas()
		require.NoError(err)

		ctx = protocol.WithActionCtx(chainCtx, protocol.ActionCtx{
			Caller:       v.caller,
			IntrinsicGas: gas,
		})
		ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
			BlockHeight: 1,
			Producer:    identityset.Address(27),
			GasLimit:    testutil.TestGasLimit,
		})

		sender, err := accountutil.AccountState(sm, v.caller)
		require.NoError(err)
		addr, err := address.FromString(v.recipient)
		require.NoError(err)
		recipient, err := accountutil.AccountState(sm, addr)
		require.NoError(err)
		gasFee := new(big.Int).Mul(v.gasPrice, new(big.Int).SetUint64(gas))

		ctx = protocol.WithFeatureCtx(ctx)
		receipt, err := p.Handle(ctx, tsf, sm)
		require.Equal(v.err, errors.Cause(err))
		if err != nil {
			require.Nil(receipt)
			// sender balance/nonce remains the same in case of error
			newSender, err := accountutil.AccountState(sm, v.caller)
			require.NoError(err)
			require.Equal(sender.Balance, newSender.Balance)
			require.Equal(sender.PendingNonce(), newSender.PendingNonce())
			continue
		}
		require.Equal(v.status, receipt.Status)

		// amount is transferred only upon success and for non-contract recipient
		if receipt.Status == uint64(iotextypes.ReceiptStatus_Success) && !v.isContract {
			gasFee.Add(gasFee, v.amount)
			// verify recipient
			addr, err := address.FromString(v.recipient)
			require.NoError(err)
			newRecipient, err := accountutil.AccountState(sm, addr)
			require.NoError(err)
			require.NoError(recipient.AddBalance(v.amount))
			require.Equal(recipient.Balance, newRecipient.Balance)
		}
		// verify sender balance/nonce
		newSender, err := accountutil.AccountState(sm, v.caller)
		require.NoError(err)
		require.NoError(sender.SubBalance(gasFee))
		require.Equal(sender.Balance, newSender.Balance)
		require.Equal(v.nonce+1, newSender.PendingNonce())

		// verify transaction log
		tLog := block.ReceiptTransactionLog(receipt)
		if tLog != nil {
			require.NotNil(tLog)
			pbLog := tLog.Proto()
			require.EqualValues(v.contractLog, pbLog.NumTransactions)
			// TODO: verify gas transaction log
			if len(pbLog.Transactions) > 1 {
				rec := pbLog.Transactions[0]
				require.Equal(v.amount.String(), rec.Amount)
				require.Equal(v.caller.String(), rec.Sender)
				require.Equal(v.recipient, rec.Recipient)
				require.Equal(iotextypes.TransactionLogType_NATIVE_TRANSFER, rec.Type)
			}
		}
	}
}
