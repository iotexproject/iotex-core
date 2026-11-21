// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
	"github.com/iotexproject/go-pkgs/crypto"
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
			r.NoError(container.Unfold(selp, context.Background(), isContract))
			// the unfolded action is what a block stores; it must decode back
			// to the same sender
			unfolded, err := deser.ActionToSealedEnvelope(selp.Proto())
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

func TestTxContainerCostIncludesBlobFee(t *testing.T) {
	r := require.New(t)
	to := common.HexToAddress("0x1234567890123456789012345678901234567890")
	tx := types.NewTx(&types.BlobTx{
		ChainID:    uint256.NewInt(4689),
		Nonce:      1,
		GasTipCap:  uint256.NewInt(1),
		GasFeeCap:  uint256.NewInt(10),
		Gas:        21000,
		To:         to,
		Value:      uint256.NewInt(5),
		BlobFeeCap: uint256.NewInt(3),
		BlobHashes: []common.Hash{{1}, {2}},
	})
	cost, err := (&txContainer{tx: tx}).Cost()
	r.NoError(err)
	want := new(big.Int).SetUint64(21000*10 + 5 + 3*2*params.BlobTxBlobGasPerBlob)
	r.Equal(want, cost)

	// no blob fee for other tx types
	tx = types.NewTx(&types.DynamicFeeTx{ChainID: big.NewInt(4689), Nonce: 1, GasTipCap: big.NewInt(1), GasFeeCap: big.NewInt(10), Gas: 21000, To: &to, Value: big.NewInt(5)})
	cost, err = (&txContainer{tx: tx}).Cost()
	r.NoError(err)
	r.Equal(big.NewInt(21000*10+5), cost)
}

const _roundTripEvmNetworkID = 4689

type roundTripCase struct {
	name  string
	to    *common.Address
	value *big.Int
	data  []byte
	ok    bool
}

// roundTripCases returns txs to the staking/rewarding protocol that do or do
// not re-encode to themselves once unfolded, plus lossless non-system txs
func roundTripCases(t *testing.T) []roundTripCase {
	r := require.New(t)
	unstake, err := NewUnstake(1, []byte("payload")).EthData()
	r.NoError(err)
	claim, err := NewClaimFromRewardingFund(big.NewInt(100), nil, nil).EthData()
	r.NoError(err)
	register, err := NewCandidateRegisterWithBLS("cand", identityset.Address(2).String(), identityset.Address(3).String(),
		identityset.Address(1).String(), "1200000000000000000000000", 1, true, popTestPubKey(t), nil, nil)
	r.NoError(err)
	registerData, err := register.EthData()
	r.NoError(err)
	registerValue, _ := new(big.Int).SetString("1200000000000000000000000", 10)
	var (
		pad       = make([]byte, 32)
		staking   = _stakingProtocolEthAddr
		rewarding = _rewardingProtocolEthAddr
		eoa       = common.BytesToAddress(identityset.Address(5).Bytes())
		contract  = common.BytesToAddress(identityset.Address(6).Bytes())
		trailing  = func(b []byte) []byte { return append(append([]byte{}, b...), pad...) }
	)
	return []roundTripCase{
		{"unstake", &staking, big.NewInt(0), unstake, true},
		{"unstake with trailing bytes", &staking, big.NewInt(0), trailing(unstake), false},
		{"unstake with value", &staking, big.NewInt(1), unstake, false},
		{"claim reward", &rewarding, big.NewInt(0), claim, true},
		{"claim reward with trailing bytes", &rewarding, big.NewInt(0), trailing(claim), false},
		{"claim reward with value", &rewarding, big.NewInt(1), claim, false},
		{"candidate register with value", &staking, registerValue, registerData, true},
		{"transfer with value and data", &eoa, big.NewInt(1), trailing(unstake), true},
		{"execution with value and data", &contract, big.NewInt(1), trailing(claim), true},
		{"contract creation", nil, big.NewInt(1), pad, true},
	}
}

func signRoundTripTx(legacy bool, c roundTripCase) *types.Transaction {
	sk := identityset.PrivateKey(1).EcdsaPrivateKey().(*ecdsa.PrivateKey)
	if legacy {
		return types.MustSignNewTx(sk, types.NewEIP155Signer(big.NewInt(_roundTripEvmNetworkID)), &types.LegacyTx{
			Nonce: 1, GasPrice: big.NewInt(1), Gas: 100000, To: c.to, Value: c.value, Data: c.data,
		})
	}
	return types.MustSignNewTx(sk, types.LatestSignerForChainID(big.NewInt(_roundTripEvmNetworkID)), &types.DynamicFeeTx{
		ChainID: big.NewInt(_roundTripEvmNetworkID), Nonce: 1, GasTipCap: big.NewInt(1), GasFeeCap: big.NewInt(2),
		Gas: 100000, To: c.to, Value: c.value, Data: c.data,
	})
}

func containerProto(t *testing.T, tx *types.Transaction) *iotextypes.Action {
	r := require.New(t)
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

func TestTxContainerLoadProtoRoundTrip(t *testing.T) {
	deser := (&Deserializer{}).SetEvmNetworkID(_roundTripEvmNetworkID)
	for _, c := range roundTripCases(t) {
		for _, legacy := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s legacy=%v", c.name, legacy), func(t *testing.T) {
				r := require.New(t)
				selp, err := deser.ActionToSealedEnvelope(containerProto(t, signRoundTripTx(legacy, c)))
				if !c.ok {
					r.ErrorIs(err, ErrInvalidAct)
					r.ErrorContains(err, "unfolded action does not match the signed transaction")
					return
				}
				r.NoError(err)
				r.NoError(selp.VerifySignature())
			})
		}
	}
	t.Run("staking with undecodable data", func(t *testing.T) {
		to := _stakingProtocolEthAddr
		_, err := deser.ActionToSealedEnvelope(containerProto(t, signRoundTripTx(false, roundTripCase{to: &to, value: big.NewInt(0)})))
		require.ErrorIs(t, err, ErrInvalidAct)
	})
}

func TestTxContainerUnfoldRoundTrip(t *testing.T) {
	deser := (&Deserializer{}).SetEvmNetworkID(_roundTripEvmNetworkID)
	contract := common.BytesToAddress(identityset.Address(6).Bytes())
	checker := func(_ context.Context, to *common.Address) (bool, bool, bool, error) {
		if to == nil {
			return true, false, false, nil
		}
		return *to == contract, *to == _stakingProtocolEthAddr, *to == _rewardingProtocolEthAddr, nil
	}
	for _, c := range roundTripCases(t) {
		for _, legacy := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s legacy=%v", c.name, legacy), func(t *testing.T) {
				r := require.New(t)
				// build the envelope directly: LoadProto refuses lossy containers
				tx := signRoundTripTx(legacy, c)
				pb := containerProto(t, tx)
				pubkey, err := crypto.BytesToPublicKey(pb.SenderPubKey)
				r.NoError(err)
				selp := &SealedEnvelope{
					Envelope:     &txContainer{chainID: 1, raw: pb.Core.GetTxContainer().Raw, tx: tx},
					encoding:     iotextypes.Encoding_TX_CONTAINER,
					evmNetworkID: _roundTripEvmNetworkID,
					srcPubkey:    pubkey,
					signature:    pb.Signature,
				}
				h, err := selp.Hash()
				r.NoError(err)
				sender := selp.SenderAddress().String()
				container := selp.Envelope.(TxContainer)

				err = container.Unfold(selp, context.Background(), checker)
				if !c.ok {
					r.ErrorIs(err, ErrInvalidAct)
					r.ErrorContains(err, "unfolded action does not match the signed transaction")
					// selp is left untouched
					r.Equal(container, selp.Envelope)
					r.EqualValues(iotextypes.Encoding_TX_CONTAINER, selp.Encoding())
					h2, err := selp.Hash()
					r.NoError(err)
					r.Equal(h, h2)
					return
				}
				r.NoError(err)
				// the unfolded action is what a block stores; it must decode back
				// to the same sender and hash
				unfolded, err := deser.ActionToSealedEnvelope(selp.Proto())
				r.NoError(err)
				r.Equal(sender, unfolded.SenderAddress().String())
				h2, err := unfolded.Hash()
				r.NoError(err)
				r.Equal(h, h2)
			})
		}
	}
}
