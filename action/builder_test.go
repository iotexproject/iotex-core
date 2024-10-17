// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"crypto/ecdsa"
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	ethercrypto "github.com/ethereum/go-ethereum/crypto"
	iotexcrypto "github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"

	. "github.com/iotexproject/iotex-core/v2/pkg/util/assertions"
	"github.com/iotexproject/iotex-core/v2/pkg/version"
)

func TestBuilderEthAddr(t *testing.T) {
	r := require.New(t)

	r.Equal(address.StakingProtocolAddrHash[:], _stakingProtocolEthAddr.Bytes())
	r.Equal(address.RewardingProtocolAddrHash[:], _rewardingProtocolEthAddr.Bytes())
}

func TestActionBuilder(t *testing.T) {
	r := require.New(t)

	bd := &Builder{}
	act := bd.SetVersion(version.ProtocolVersion).
		SetNonce(2).
		SetGasLimit(10003).
		SetGasPrice(big.NewInt(10004)).
		Build()
	r.Equal(uint32(version.ProtocolVersion), act.version)
	r.Equal(uint64(2), act.Nonce())
	r.Equal(uint64(10003), act.Gas())
	r.Equal(big.NewInt(10004), act.GasPrice())
}

func TestBuildRewardingAction(t *testing.T) {
	r := require.New(t)

	eb := &EnvelopeBuilder{}
	eb.SetChainID(2)

	t.Run("UnexpectToAddress", func(t *testing.T) {
		to := common.HexToAddress("0x0000000000000000000000000000000000000001")
		calldata := []byte("any")

		elp, err := eb.BuildRewardingAction(types.NewTx(&types.LegacyTx{
			Nonce:    1,
			GasPrice: big.NewInt(1),
			Gas:      1,
			To:       &to,
			Value:    big.NewInt(1),
			Data:     calldata,
		}))
		r.Nil(elp)
		r.ErrorIs(err, ErrInvalidAct)
	})

	t.Run("InvalidMethodSignature", func(t *testing.T) {
		// method signature should in ('claim', 'deposit')
		to := _rewardingProtocolEthAddr
		elp, err := eb.BuildRewardingAction(types.NewTx(&types.LegacyTx{
			Nonce:    1,
			GasPrice: big.NewInt(1),
			Gas:      1,
			To:       &to,
			Value:    big.NewInt(1),
			Data:     []byte("InvalidMethodSig"),
		}))
		r.Nil(elp)
		r.ErrorIs(err, ErrInvalidABI)
	})

	t.Run("Success", func(t *testing.T) {
		method := _claimRewardingMethodV1
		t.Run("ClaimRewarding", func(t *testing.T) {
			inputs := MustNoErrorV(method.Inputs.Pack(big.NewInt(101), []byte("any")))
			elp, err := eb.BuildRewardingAction(types.NewTx(&types.LegacyTx{
				Nonce:    1,
				GasPrice: big.NewInt(10004),
				Gas:      10000,
				To:       &_rewardingProtocolEthAddr,
				Value:    big.NewInt(100),
				Data:     append(method.ID, inputs...),
			}))
			r.NoError(err)
			r.IsType(&ClaimFromRewardingFund{}, elp.Action())
			r.Equal(big.NewInt(10004), elp.GasPrice())
			r.Equal(uint64(10000), elp.Gas())
			r.Equal(big.NewInt(101), elp.Action().(*ClaimFromRewardingFund).ClaimAmount())
		})
		t.Run("Debug", func(t *testing.T) {
			eb := &EnvelopeBuilder{}
			eb.SetChainID(4689)

			inputs := MustNoErrorV(method.Inputs.Pack(big.NewInt(100), []byte("any")))
			to := common.HexToAddress("0xA576C141e5659137ddDa4223d209d4744b2106BE")
			tx := types.NewTx(&types.LegacyTx{
				Nonce:    0,
				GasPrice: big.NewInt(100),
				Gas:      21000,
				To:       &to,
				Value:    big.NewInt(0),
				Data:     append(method.ID, inputs...),
			})
			t.Log(tx.Hash().String())
			raw, err := tx.MarshalBinary()
			r.NoError(err)
			t.Log(hex.EncodeToString(raw))
		})
		method = _depositRewardMethod
		t.Run("DepositRewarding", func(t *testing.T) {
			inputs := MustNoErrorV(method.Inputs.Pack(big.NewInt(102), []byte("any")))
			elp, err := eb.BuildRewardingAction(types.NewTx(&types.LegacyTx{
				Nonce:    1,
				GasPrice: big.NewInt(10004),
				Gas:      10000,
				To:       &_rewardingProtocolEthAddr,
				Value:    big.NewInt(100),
				Data:     append(method.ID, inputs...),
			}))
			r.NoError(err)
			r.IsType(&DepositToRewardingFund{}, elp.Action())
			r.EqualValues(big.NewInt(10004), elp.GasPrice())
			r.EqualValues(10000, elp.Gas())
			r.EqualValues(big.NewInt(102), elp.Action().(*DepositToRewardingFund).Amount())
		})
	})
}

func TestEthTxUtils(t *testing.T) {
	r := require.New(t)
	var (
		skhex   = "708361c6460c93f027df0823eb440f3270ee2937944f2de933456a3900d6dd1a"
		sk1, _  = iotexcrypto.HexStringToPrivateKey(skhex)
		sk2, _  = ethercrypto.HexToECDSA(skhex)
		chainID = uint32(4689)
	)

	pk1 := sk1.PublicKey()
	iotexhexaddr := hex.EncodeToString(pk1.Bytes())

	pk2 := sk2.Public().(*ecdsa.PublicKey)
	etherhexaddr := hex.EncodeToString(ethercrypto.FromECDSAPub(pk2))

	r.Equal(iotexhexaddr, etherhexaddr)

	addr, err := address.FromHex("0xA576C141e5659137ddDa4223d209d4744b2106BE")
	r.NoError(err)
	act := NewClaimFromRewardingFund(big.NewInt(1), addr, []byte("any"))
	elp := (&EnvelopeBuilder{}).SetNonce(100).SetGasLimit(21000).
		SetGasPrice(big.NewInt(101)).SetAction(act).Build()
	tx, err := elp.ToEthTx(chainID, iotextypes.Encoding_ETHEREUM_EIP155)
	r.NoError(err)

	var (
		signer1, _ = NewEthSigner(iotextypes.Encoding_ETHEREUM_EIP155, chainID)
		sig1, _    = sk1.Sign(tx.Hash().Bytes())
		signer2    = types.NewCancunSigner(big.NewInt(int64(chainID)))
		sig2, _    = ethercrypto.Sign(tx.Hash().Bytes(), sk2)
	)
	r.Equal(signer1, signer2)
	r.Equal(sig1, sig2)

	tx1, _ := RawTxToSignedTx(tx, signer1, sig1)
	tx2, _ := tx.WithSignature(signer2, sig2)
	r.Equal(tx1.Hash(), tx2.Hash())

	sender1, _ := signer1.Sender(tx1)
	sender2, _ := signer2.Sender(tx2)
	r.Equal(sender1, sender2)

	pk1recover, _ := iotexcrypto.RecoverPubkey(signer1.Hash(tx).Bytes(), sig1)
	pk2recoverbytes, _ := ethercrypto.Ecrecover(signer2.Hash(tx).Bytes(), sig2)
	r.Equal(pk1recover.Bytes(), pk2recoverbytes)
	pk2recover, _ := ethercrypto.SigToPub(signer2.Hash(tx).Bytes(), sig2)

	r.Equal(ethercrypto.FromECDSAPub(pk2recover), pk2recoverbytes)
}
