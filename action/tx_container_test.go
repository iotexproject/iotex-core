// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"context"
	"crypto/ecdsa"
	"math"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

func TestTxContainerSetCodeAuthorizationChainID(t *testing.T) {
	r := require.New(t)
	const evmNetworkID = 4689
	var (
		sk     = identityset.PrivateKey(1).EcdsaPrivateKey().(*ecdsa.PrivateKey)
		to     = common.HexToAddress("0x1234567890123456789012345678901234567890")
		signer = types.LatestSignerForChainID(big.NewInt(evmNetworkID))
		deser  = (&Deserializer{}).SetEvmNetworkID(evmNetworkID)
	)
	newContainer := func(authChainID *uint256.Int) *iotextypes.Action {
		auth, err := types.SignSetCode(sk, types.SetCodeAuthorization{
			ChainID: *authChainID,
			Address: common.Address(identityset.Address(5).Bytes()),
			Nonce:   3,
		})
		r.NoError(err)
		tx := types.MustSignNewTx(sk, signer, &types.SetCodeTx{
			ChainID:   uint256.NewInt(evmNetworkID),
			Nonce:     1,
			GasTipCap: uint256.NewInt(1),
			GasFeeCap: uint256.NewInt(2),
			Gas:       100000,
			To:        to,
			Value:     uint256.NewInt(0),
			AuthList:  []types.SetCodeAuthorization{auth},
		})
		raw, err := tx.MarshalBinary()
		r.NoError(err)
		_, sig, pubkey, err := ExtractTypeSigPubkey(tx)
		r.NoError(err)
		return &iotextypes.Action{
			Core: &iotextypes.ActionCore{
				ChainID: 1,
				Action:  &iotextypes.ActionCore_TxContainer{TxContainer: &iotextypes.TxContainer{Raw: raw}},
			},
			SenderPubKey: pubkey.Bytes(),
			Signature:    sig,
			Encoding:     iotextypes.Encoding_TX_CONTAINER,
		}
	}
	isContract := func(context.Context, *common.Address) (bool, bool, bool, error) {
		return true, false, false, nil
	}

	for _, c := range []struct {
		name    string
		chainID *uint256.Int
	}{
		{"wildcard", uint256.NewInt(0)},
		{"network", uint256.NewInt(evmNetworkID)},
		{"max uint32", uint256.NewInt(math.MaxUint32)},
	} {
		t.Run(c.name, func(t *testing.T) {
			selp, err := deser.ActionToSealedEnvelope(newContainer(c.chainID))
			r.NoError(err)
			sender := selp.SenderAddress().String()
			container, ok := selp.Envelope.(TxContainer)
			r.True(ok)
			unfoldedSelp, err := container.Unfold(selp, context.Background(), isContract)
			r.NoError(err)
			// the unfolded action is what a block stores; it must decode back
			// to the same sender
			unfolded, err := deser.ActionToSealedEnvelope(unfoldedSelp.Proto())
			r.NoError(err)
			r.Equal(sender, unfolded.SenderAddress().String())
		})
	}

	for _, c := range []struct {
		name    string
		chainID *uint256.Int
	}{
		{"2^32", new(uint256.Int).Lsh(uint256.NewInt(1), 32)},
		{"2^32 + network", new(uint256.Int).Add(new(uint256.Int).Lsh(uint256.NewInt(1), 32), uint256.NewInt(evmNetworkID))},
		{"2^64 + network", new(uint256.Int).Add(new(uint256.Int).Lsh(uint256.NewInt(1), 64), uint256.NewInt(evmNetworkID))},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, err := deser.ActionToSealedEnvelope(newContainer(c.chainID))
			r.ErrorIs(err, ErrInvalidAct)
		})
	}
}

func TestTxContainerOversizedSignatureValues(t *testing.T) {
	r := require.New(t)
	to := common.HexToAddress("0x1234567890123456789012345678901234567890")
	oversized := new(big.Int).Lsh(big.NewInt(1), 256) // 33 bytes
	for _, c := range []struct {
		name  string
		inner func(r, s *big.Int) types.TxData
	}{
		{"legacy", func(r, s *big.Int) types.TxData {
			return &types.LegacyTx{Nonce: 1, GasPrice: big.NewInt(1), Gas: 21000, To: &to, Value: big.NewInt(0), V: big.NewInt(27), R: r, S: s}
		}},
		{"dynamic fee", func(r, s *big.Int) types.TxData {
			return &types.DynamicFeeTx{ChainID: big.NewInt(4689), Nonce: 1, GasTipCap: big.NewInt(1), GasFeeCap: big.NewInt(1), Gas: 21000, To: &to, Value: big.NewInt(0), V: big.NewInt(0), R: r, S: s}
		}},
	} {
		for _, sig := range []struct {
			name string
			r, s *big.Int
		}{
			{"R", oversized, big.NewInt(1)},
			{"S", big.NewInt(1), oversized},
		} {
			t.Run(c.name+" oversized "+sig.name, func(t *testing.T) {
				raw, err := types.NewTx(c.inner(sig.r, sig.s)).MarshalBinary()
				r.NoError(err)
				tx := types.Transaction{}
				r.NoError(tx.UnmarshalBinary(raw))
				_, gotR, gotS := tx.RawSignatureValues()
				r.True(gotR.BitLen() > 256 || gotS.BitLen() > 256, "RLP decoding should preserve the oversized value")

				_, _, _, err = ExtractTypeSigPubkey(&tx)
				r.ErrorIs(err, ErrNotSupported)

				err = (&txContainer{}).LoadProto(&iotextypes.ActionCore{
					ChainID: 4689,
					Action:  &iotextypes.ActionCore_TxContainer{TxContainer: &iotextypes.TxContainer{Raw: raw}},
				})
				r.ErrorIs(err, ErrInvalidAct)
			})
		}
	}
}
