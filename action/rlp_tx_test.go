// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	. "github.com/iotexproject/iotex-core/v2/pkg/util/assertions"
)

func TestGenerateRlp(t *testing.T) {
	require := require.New(t)

	ab := AbstractAction{
		version:  1,
		nonce:    2,
		gasLimit: 1000,
		gasPrice: new(big.Int),
	}
	builder := (&EnvelopeBuilder{}).SetNonce(ab.Nonce()).
		SetGasLimit(ab.Gas()).SetGasPrice(ab.GasPrice())
	for _, v := range []struct {
		act  actionPayload
		sig  []byte
		err  string
		hash hash.Hash256
	}{
		{&Transfer{
			recipient: "io1x9qa70ewgs24xwak66lz5dgm9ku7ap80vw3071",
		}, _validSig, address.ErrInvalidAddr.Error(), hash.ZeroHash256},
		{&Transfer{
			amount:    big.NewInt(100),
			recipient: "",
		}, _signByte, "address length = 0, expecting 41", hash.ZeroHash256},
		{&Transfer{
			amount:    big.NewInt(100),
			recipient: "",
		}, _validSig, "", hash.BytesToHash256(MustNoErrorV(hex.DecodeString("87e39e819193ae46472eb1320739b34c4c3b38ea321c7cc503432bdcfd0cbf15")))},
		{&Transfer{
			recipient: "io1x9qa70ewgs24xwak66lz5dgm9ku7ap80vw3070",
		}, _validSig, "", hash.BytesToHash256(MustNoErrorV(hex.DecodeString("eaaf38a552809a9bdb1509c8093bd2c74eb07baff862dae692c1d2b865478b14")))},
		{&Execution{
			amount: big.NewInt(100),
			data:   _signByte,
		}, _validSig, "", hash.BytesToHash256(MustNoErrorV(hex.DecodeString("fcdd0c3d07f438d6e67ea852b40e5dc256d75f5e1fa9ac3ca96030efeb634150")))},
		{&Execution{
			contract: "io1x9qa70ewgs24xwak66lz5dgm9ku7ap80vw3070",
			amount:   big.NewInt(100),
			data:     _signByte,
		}, _validSig, "", hash.BytesToHash256(MustNoErrorV(hex.DecodeString("fee3db88ee7d7defa9eded672d08fc8641f760f3a11d404a53276ad6f412b8a5")))},
	} {
		elp := builder.SetAction(v.act).Build()
		tx, err := elp.ToEthTx(0, iotextypes.Encoding_ETHEREUM_EIP155)
		if err != nil {
			require.Contains(err.Error(), v.err)
			continue
		}
		signer, err := NewEthSigner(iotextypes.Encoding_ETHEREUM_EIP155, _evmNetworkID)
		require.NoError(err)
		h, err := rlpSignedHash(tx, signer, v.sig)
		if err != nil {
			require.Contains(err.Error(), v.err)
		}
		require.Equal(v.hash, h)
	}
}

type rlpTest struct {
	actType       string
	nonce, limit  uint64
	price, amount string
	to            string
	chainID       uint32
	encoding      iotextypes.Encoding
	data          []byte
	hash          string
}

var (
	// deterministic deployment: https://github.com/Arachnid/deterministic-deployment-proxy
	// see example at https://etherscan.io/tx/0xeddf9e61fb9d8f5111840daef55e5fde0041f5702856532cdbb5a02998033d26
	deterministicDeploymentTx = "f8a58085174876e800830186a08080b853604580600e600039806000f350fe7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe03601600081602082378035828234f58015156039578182fd5b8082525050506014600cf31ba02222222222222222222222222222222222222222222222222222222222222222a02222222222222222222222222222222222222222222222222222222222222222"
	deterministicDeployer     = "0x3fab184622dc19b6109349b94811493bf2a45362"

	// example access list tx, created with a random chainID value = 0x7a69 = 31337
	accessListTx = "01f8a0827a690184ee6b280082520894a0ee7a142d267c1f36714e4a8f75612f20a797206480f838f794a0ee7a142d267c1f36714e4a8f75612f20a79720e1a0000000000000000000000000000000000000000000000000000000000000000001a0eb211dfd353d76d43ea31a130ff36ac4eb7b379eae4d49fa2376741daf32f9ffa07ab673241d75e103f81ddd4aa34dd6849faf2f0f593eebe61a68fed74490a348"

	rlpTests = []rlpTest{
		{
			"transfer", 0, 21000, "1000000000000", "100000000000000000",
			"0x3141df3f2e4415533bb6d6be2A351B2db9ee84EF",
			_evmNetworkID,
			iotextypes.Encoding_ETHEREUM_EIP155,
			nil,
			"13d18bcf4ba5bfad081943dce8f971df64bf7abe64cf194d2d29f57d317647af",
		},
		{
			"execution", 13, 21000, "1000000000000", "0",
			"0xAC7AC39De679b19AaE042c0cE19fAcB86e0A4117",
			_evmNetworkID,
			iotextypes.Encoding_ETHEREUM_EIP155,
			MustNoErrorV(hex.DecodeString("a9059cbb0000000000000000000000003141df3f2e4415533bb6d6be2a351b2db9ee84ef000000000000000000000000000000000000000000000000000000003b9aca00")),
			"adc07bf218c539b5289dfa3f79424c0810d92228f4ac9f9316f02bc7d3a3fe73",
		},
		{
			"execution", 46, 8500000, "1000000", "0",
			EmptyAddress,
			_evmNetworkID,
			iotextypes.Encoding_ETHEREUM_EIP155,
			MustNoErrorV(hex.DecodeString("608060405234801561001057600080fd5b50336000806101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555061019c806100606000396000f3fe608060405234801561001057600080fd5b50600436106100415760003560e01c8063445df0ac146100465780638da5cb5b14610064578063fdacd576146100ae575b600080fd5b61004e6100dc565b6040518082815260200191505060405180910390f35b61006c6100e2565b604051808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200191505060405180910390f35b6100da600480360360208110156100c457600080fd5b8101908080359060200190929190505050610107565b005b60015481565b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1681565b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff16141561016457806001819055505b5056fea265627a7a72315820e54fe55a78b9d8bec22b4d3e6b94b7e59799daee3940423eb1aa30fe643eeb9a64736f6c63430005100032")),
			"499e23471817b1d692c15f184fd2972135154ef745f5d694a160a58e56c592d1",
		},
		{
			"stakeCreate", 9, 36000, "0", "0",
			"0x04C22AfaE6a03438b8FED74cb1Cf441168DF3F12",
			_evmNetworkID,
			iotextypes.Encoding_ETHEREUM_EIP155,
			MustNoErrorV(hex.DecodeString("a3d374c400000000000000000000000000000000000000000000000000000000000000a000000000000000000000000000000000000000000000000000000000000000640000000000000000000000000000000000000000000000000000000000000007000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000e0000000000000000000000000000000000000000000000000000000000000000474657374000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000")),
			"feb7c06ca6f4528318f93dc5a81d7561fb98f1f2f7e44e4c10f5b1c04ceb7ebf",
		},
		{
			"stakeAddDeposit", 9, 23200, "0", "0",
			"0x04C22AfaE6a03438b8FED74cb1Cf441168DF3F12",
			_evmNetworkID,
			iotextypes.Encoding_ETHEREUM_EIP155,
			MustNoErrorV(hex.DecodeString("34e8e1450000000000000000000000000000000000000000000000000000000000000007000000000000000000000000000000000000000000000000000000000000006400000000000000000000000000000000000000000000000000000000000000600000000000000000000000000000000000000000000000000000000000000000")),
			"ed2cceb7c021e30c94fa351af9f28908a97feb1c9572401668cb7622198902d0",
		},
		{
			"changeCandidate", 9, 29600, "0", "0",
			"0x04C22AfaE6a03438b8FED74cb1Cf441168DF3F12",
			_evmNetworkID,
			iotextypes.Encoding_ETHEREUM_EIP155,
			MustNoErrorV(hex.DecodeString("fb3d51380000000000000000000000000000000000000000000000000000000000000060000000000000000000000000000000000000000000000000000000000000000700000000000000000000000000000000000000000000000000000000000000a0000000000000000000000000000000000000000000000000000000000000000474657374000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000")),
			"bc9a3aa241e09309ddb252e7e256655a9f401f6956cec66b81a67b623959a8db",
		},
		{
			"unstake", 9, 21000, "0", "0",
			"0x04C22AfaE6a03438b8FED74cb1Cf441168DF3F12",
			_evmNetworkID,
			iotextypes.Encoding_ETHEREUM_EIP155,
			MustNoErrorV(hex.DecodeString("2bde151d000000000000000000000000000000000000000000000000000000000000000700000000000000000000000000000000000000000000000000000000000000400000000000000000000000000000000000000000000000000000000000000000")),
			"a743d5ada3820734fa767f021d38d270b9c99d0f514f0cf67f052d97e4ebdf5a",
		},
		{
			"withdrawStake", 9, 21000, "0", "0",
			"0x04C22AfaE6a03438b8FED74cb1Cf441168DF3F12",
			_evmNetworkID,
			iotextypes.Encoding_ETHEREUM_EIP155,
			MustNoErrorV(hex.DecodeString("d179ffb5000000000000000000000000000000000000000000000000000000000000000700000000000000000000000000000000000000000000000000000000000000400000000000000000000000000000000000000000000000000000000000000000")),
			"f91d8be1c8577ca3dfc496d3e6d6d5dd28a576dab0bfd0ca9b1d9409e7d3d22b",
		},
		{
			"restake", 9, 26400, "0", "0",
			"0x04C22AfaE6a03438b8FED74cb1Cf441168DF3F12",
			_evmNetworkID,
			iotextypes.Encoding_ETHEREUM_EIP155,
			MustNoErrorV(hex.DecodeString("4c4fee4b00000000000000000000000000000000000000000000000000000000000000070000000000000000000000000000000000000000000000000000000000000007000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000800000000000000000000000000000000000000000000000000000000000000000")),
			"6491f1f36f931e215961647af1ad0307cfa8f2f636c532601ae2edfaac42892c",
		},
		{
			"transferStake", 9, 23200, "0", "0",
			"0x04C22AfaE6a03438b8FED74cb1Cf441168DF3F12",
			_evmNetworkID,
			iotextypes.Encoding_ETHEREUM_EIP155,
			MustNoErrorV(hex.DecodeString("9cb560bb0000000000000000000000003041a575c7a70021e3082929798c8c3fdaa9d824000000000000000000000000000000000000000000000000000000000000000700000000000000000000000000000000000000000000000000000000000000600000000000000000000000000000000000000000000000000000000000000000")),
			"f1c9887fc7753c940ec183ac50fa4427b74b2da40d414fa3a09d22c21beee477",
		},
		{
			"candidateRegister", 9, 21000, "0", "0",
			"0x04C22AfaE6a03438b8FED74cb1Cf441168DF3F12",
			_evmNetworkID,
			iotextypes.Encoding_ETHEREUM_EIP155,
			MustNoErrorV(hex.DecodeString("bee5f7b700000000000000000000000000000000000000000000000000000000000001000000000000000000000000003041a575c7a70021e3082929798c8c3fdaa9d8240000000000000000000000003041a575c7a70021e3082929798c8c3fdaa9d8240000000000000000000000003041a575c7a70021e3082929798c8c3fdaa9d8240000000000000000000000000000000000000000000000000000000000000064000000000000000000000000000000000000000000000000000000000000000700000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000140000000000000000000000000000000000000000000000000000000000000000474657374000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000")),
			"ebced4215d13d4019905a81c62eee03c7b97194f68c33afb3d3c5411aeb173af",
		},
		{
			"candidateUpdate", 9, 21000, "0", "0",
			"0x04C22AfaE6a03438b8FED74cb1Cf441168DF3F12",
			_evmNetworkID,
			iotextypes.Encoding_ETHEREUM_EIP155,
			MustNoErrorV(hex.DecodeString("435f9f2200000000000000000000000000000000000000000000000000000000000000600000000000000000000000003041a575c7a70021e3082929798c8c3fdaa9d8240000000000000000000000003041a575c7a70021e3082929798c8c3fdaa9d82400000000000000000000000000000000000000000000000000000000000000047465737400000000000000000000000000000000000000000000000000000000")),
			"250ac8884628496a5db7bc909c3aac3b79f0e17f8b7b30194c81985f26a110c0",
		},
		{
			"rewardingClaim", 0, 21000, "100", "0",
			"0xA576C141e5659137ddDa4223d209d4744b2106BE",
			_evmNetworkID,
			iotextypes.Encoding_ETHEREUM_EIP155,
			MustNoErrorV(hex.DecodeString("2df163ef000000000000000000000000000000000000000000000000000000000000006500000000000000000000000000000000000000000000000000000000000000400000000000000000000000000000000000000000000000000000000000000000")),
			"9b58a9cc739ee9b8eced536509f969a9e3d4ba760bf829a4607a41bb31757b3c",
		},
		{
			"rewardingClaimWithAddress", 5, 21000, "100000", "0",
			"0xA576C141e5659137ddDa4223d209d4744b2106BE",
			_evmNetworkID,
			iotextypes.Encoding_ETHEREUM_EIP155,
			MustNoErrorV(hex.DecodeString("d804b87c0000000000000000000000000000000000000000000000000000002e90edd000000000000000000000000000000000000000000000000000000000000000006000000000000000000000000000000000000000000000000000000000000000c00000000000000000000000000000000000000000000000000000000000000029696f313433617638383078307863653474737939737877723861766870687135736768756d3737637400000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000")),
			"fd2abe823ff00b46d7bf458c8dfca0671d83dd98422add134156e7ca36dadf17",
		},
		{
			"rewardingDeposit", 1, 21000, "100", "0",
			"0xA576C141e5659137ddDa4223d209d4744b2106BE",
			_evmNetworkID,
			iotextypes.Encoding_ETHEREUM_EIP155,
			MustNoErrorV(hex.DecodeString("27852a6b000000000000000000000000000000000000000000000000000000000000006500000000000000000000000000000000000000000000000000000000000000400000000000000000000000000000000000000000000000000000000000000000")),
			"5381613552c4265523455e8b613e7248af68f6889e0edd431759442b89cb1316",
		},
		{
			"candidateActivate", 120, 21000, "1000000000000", "0",
			"0x04C22AfaE6a03438b8FED74cb1Cf441168DF3F12",
			_evmNetworkID,
			iotextypes.Encoding_ETHEREUM_EIP155,
			MustNoErrorV(hex.DecodeString("ef68b1a40000000000000000000000000000000000000000000000000000000000000001")),
			"9a948f0c1d20fadc0888d9b83d23e214935b87fe07aebe3eb47cf65b0e1050c0",
		},
		{
			"candidateEndorsement", 120, 21000, "1000000000000", "0",
			"0x04C22AfaE6a03438b8FED74cb1Cf441168DF3F12",
			_evmNetworkID,
			iotextypes.Encoding_ETHEREUM_EIP155,
			MustNoErrorV(hex.DecodeString("8a8a5d5100000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000001")),
			"95977fb340867fa13f787df864ed5388cbedc7f1329d5430b6c536e444a3f918",
		},
		{
			"candidateTransferOwnership", 1, 1000000, "1000", "0",
			"0x04C22AfaE6a03438b8FED74cb1Cf441168DF3F12",
			_evmNetworkID,
			iotextypes.Encoding_ETHEREUM_EIP155,
			MustNoErrorV(hex.DecodeString("3bf030350000000000000000000000007f54538b6260d754701e2f4a9a9bc0cb654293d900000000000000000000000000000000000000000000000000000000000000400000000000000000000000000000000000000000000000000000000000000000")),
			"cd19105d08b88a0ee3569297221ced1a610bea19fb21f917b012c4834be38b73",
		},
		{
			"unprotected", 0, 100000, "100000000000", "0",
			EmptyAddress,
			0,
			iotextypes.Encoding_ETHEREUM_UNPROTECTED,
			MustNoErrorV(hex.DecodeString("604580600e600039806000f350fe7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe03601600081602082378035828234f58015156039578182fd5b8082525050506014600cf3")),
			"eddf9e61fb9d8f5111840daef55e5fde0041f5702856532cdbb5a02998033d26",
		},
		{
			"migrateStake", 120, 21000, "10000", "0",
			"0x04C22AfaE6a03438b8FED74cb1Cf441168DF3F12",
			_evmNetworkID,
			iotextypes.Encoding_ETHEREUM_EIP155,
			MustNoErrorV(hex.DecodeString("c53b10a4000000000000000000000000000000000000000000000000000000000000000a")),
			"90a298cde17fa688a2c9340ed29767e72e4e24e3a5eeb0166179537f0cffeffc",
		},
		{
			"endorseCandidate", 120, 21000, "10000", "0",
			"0x04C22AfaE6a03438b8FED74cb1Cf441168DF3F12",
			_evmNetworkID,
			iotextypes.Encoding_ETHEREUM_EIP155,
			MustNoErrorV(hex.DecodeString("284ab8110000000000000000000000000000000000000000000000000000000000000002")),
			"b5402b818a5501c0fbb2b876443174709eeb74de5d3e03d41b57361dd29cbf6f",
		},
		{
			"intentToRevokeEndorsement", 120, 21000, "10000", "0",
			"0x04C22AfaE6a03438b8FED74cb1Cf441168DF3F12",
			_evmNetworkID,
			iotextypes.Encoding_ETHEREUM_EIP155,
			MustNoErrorV(hex.DecodeString("85edc03c0000000000000000000000000000000000000000000000000000000000000002")),
			"756e5842d22df08e30301d744c9dedd63e8c74ad2eb19cf2f8d7699541693bb1",
		},
		{
			"revokeEndorsement", 120, 21000, "10000", "0",
			"0x04C22AfaE6a03438b8FED74cb1Cf441168DF3F12",
			_evmNetworkID,
			iotextypes.Encoding_ETHEREUM_EIP155,
			MustNoErrorV(hex.DecodeString("120f99ad0000000000000000000000000000000000000000000000000000000000000002")),
			"0ae11105b673835c4b612f98ab748d5b4e9aa1e779f404866658476fd230c33f",
		},
		{
			"accesslist", 120, 21000, "1000000000", "0",
			"0x3141df3f2e4415533bb6d6be2A351B2db9ee84EF",
			_evmNetworkID,
			iotextypes.Encoding_ETHEREUM_EIP155,
			nil,
			"b30463d727248560e17d82764604fa64b2161f3bdb53434a6afd15fc64a42918",
		},
		{
			"dynamicfee", 120, 21000, "1000000000", "0",
			"0x3141df3f2e4415533bb6d6be2A351B2db9ee84EF",
			_evmNetworkID,
			iotextypes.Encoding_ETHEREUM_EIP155,
			nil,
			"46e367af6c609032b85e7d3f40f0af3fcb59779b55d92eefea2ee1dbdaca41fa",
		},
		{
			"blobtx", 120, 21000, "1000000000", "0",
			"0x3141df3f2e4415533bb6d6be2A351B2db9ee84EF",
			_evmNetworkID,
			iotextypes.Encoding_ETHEREUM_EIP155,
			[]byte{1, 2, 3},
			"526b46d97b334cc5edb6095448406c9df74c8d6310d22044cb9a3a2a305bc398",
		},
	}
)

func TestEthTxDecodeVerify(t *testing.T) {
	require := require.New(t)
	var (
		sk     = MustNoErrorV(crypto.HexStringToPrivateKey("a000000000000000000000000000000000000000000000000000000000000000"))
		pkhash = sk.PublicKey().Address().Hex()
	)

	checkSelp := func(selp *SealedEnvelope, tx *types.Transaction, e rlpTest) {
		h := MustNoErrorV(selp.Hash())
		require.Equal(e.hash, hex.EncodeToString(h[:]))
		if e.actType == "unprotected" {
			require.Equal(deterministicDeployer, selp.srcPubkey.Address().Hex())
		} else {
			require.Equal(pkhash, selp.srcPubkey.Address().Hex())
		}
		require.Equal(e.chainID, selp.evmNetworkID)
		require.EqualValues(1, selp.ChainID())
		require.EqualValues(tx.Type(), selp.TxType())
		require.EqualValues(tx.Nonce(), selp.Nonce())
		require.EqualValues(tx.Gas(), selp.Gas())
		require.Equal(tx.GasPrice(), selp.GasPrice())
		require.Equal(tx.AccessList(), selp.AccessList())
		require.Equal(tx.GasFeeCap(), selp.GasFeeCap())
		require.Equal(tx.GasTipCap(), selp.GasTipCap())
		require.Equal(tx.BlobGas(), selp.BlobGas())
		require.Equal(tx.BlobGasFeeCap(), selp.BlobGasFeeCap())
		require.Equal(tx.BlobHashes(), selp.BlobHashes())
		require.Equal(tx.BlobTxSidecar(), selp.BlobTxSidecar())
		raw := MustNoErrorV(selp.envelopeHash())
		signer := MustNoErrorV(NewEthSigner(e.encoding, uint32(e.chainID)))
		rawHash := signer.Hash(tx)
		require.True(bytes.Equal(rawHash[:], raw[:]))
		require.NotEqual(raw, h)
		require.NoError(selp.VerifySignature())
	}

	for _, v := range rlpTests {
		t.Run(v.actType, func(t *testing.T) {
			raw := generateRLPTestRaw(sk.EcdsaPrivateKey().(*ecdsa.PrivateKey), &v)
			// decode received RLP tx
			tx := MustNoErrorV(DecodeEtherTx(raw))
			encoding, sig, pubkey, err := ExtractTypeSigPubkey(tx)
			require.NoError(err)
			require.Equal(v.encoding, encoding)
			V, _, _ := tx.RawSignatureValues()
			recID := V.Uint64()
			if encoding == iotextypes.Encoding_ETHEREUM_UNPROTECTED {
				require.EqualValues(types.LegacyTxType, tx.Type())
				require.False(tx.Protected())
				require.Zero(tx.ChainId().Uint64())
				// unprotected tx has V = 27, 28
				require.True(recID == 27 || recID == 28)
				require.True(sig[64] == 27 || sig[64] == 28)
			} else {
				require.Equal(iotextypes.Encoding_ETHEREUM_EIP155, encoding)
				switch tx.Type() {
				case types.LegacyTxType:
					require.True(tx.Protected())
					require.Less(uint64(28), recID)
				case types.AccessListTxType, types.DynamicFeeTxType, types.BlobTxType:
					require.True(tx.Protected())
					// // accesslist tx has V = 0, 1
					require.LessOrEqual(recID, uint64(1))
					require.True(sig[64] == 27 || sig[64] == 28)
				}
			}
			require.EqualValues(v.chainID, tx.ChainId().Uint64())
			if v.actType == "unprotected" {
				require.Equal(deterministicDeployer, pubkey.Address().Hex())
			} else {
				require.Equal(pkhash, pubkey.Address().Hex())
			}
			require.Equal(v.hash, tx.Hash().Hex()[2:])

			var pb [2]*iotextypes.Action
			// convert to native proto
			pb[0] = &iotextypes.Action{
				Encoding:     encoding,
				Core:         MustNoErrorV(convertToNativeProto(tx, v.actType)),
				SenderPubKey: pubkey.Bytes(),
				Signature:    sig,
			}
			// test tx container
			pb[1] = &iotextypes.Action{
				Core:         MustNoErrorV(EthRawToContainer(1, raw)),
				SenderPubKey: pubkey.Bytes(),
				Signature:    sig,
				Encoding:     iotextypes.Encoding_TX_CONTAINER,
			}

			for i := 0; i < 2; i++ {
				// send on wire
				bs := MustNoErrorV(proto.Marshal(pb[i]))
				// receive from API
				e := &iotextypes.Action{}
				proto.Unmarshal(bs, e)

				selp := MustNoErrorV((&Deserializer{}).SetEvmNetworkID(v.chainID).ActionToSealedEnvelope(e))
				require.True(bytes.Equal(sig, selp.signature))
				checkSelp(selp, tx, v)
				if i == 0 {
					require.Equal(v.encoding, selp.encoding)
				} else {
					// tx container
					require.EqualValues(iotextypes.Encoding_TX_CONTAINER, selp.encoding)
				}

				// evm tx conversion
				if i == 1 {
					// staking and rewarding can be directly converted
					elp := MustNoErrorV(StakingRewardingTxToEnvelope(1, tx))
					var to string
					if tx.To() != nil {
						to = MustNoErrorV(address.FromBytes(tx.To().Bytes())).String()
					}
					require.Equal(to == address.StakingProtocolAddr || to == string(address.RewardingProtocol), elp != nil)
					// tx unfolding
					_, ok := selp.Action().(*txContainer)
					require.True(ok)
					container, ok := selp.Action().(TxContainer)
					require.True(ok)
					require.NoError(container.Unfold(selp, context.Background(), checkContract(v.to, v.actType)))
					require.True(bytes.Equal(sig, selp.signature))
					checkSelp(selp, tx, v)
					require.Equal(v.encoding, selp.encoding)
					// selp converted to actual tx
					_, ok = selp.Action().(TxContainer)
					require.False(ok)
					if elp != nil {
						require.Equal(elp, selp.Envelope)
					}
				}
				evmTx := MustNoErrorV(selp.ToEthTx())
				// verify against original tx
				require.Equal(tx.Type(), evmTx.Type())
				require.Equal(tx.Nonce(), evmTx.Nonce())
				require.Equal(tx.Gas(), evmTx.Gas())
				require.Equal(tx.GasPrice(), evmTx.GasPrice())
				if v.to == "" {
					require.Nil(evmTx.To())
				} else {
					require.Equal(tx.To(), evmTx.To())
				}
				require.Equal(tx.Value(), evmTx.Value())
				if v.data != nil {
					require.Equal(tx.Data(), evmTx.Data())
				} else {
					require.Zero(len(evmTx.Data()))
				}
				require.Equal(tx.Cost(), evmTx.Cost())
				require.Equal(tx.AccessList(), evmTx.AccessList())
				require.Equal(tx.GasFeeCap(), evmTx.GasFeeCap())
				require.Equal(tx.GasTipCap(), evmTx.GasTipCap())
				require.Equal(tx.BlobGas(), evmTx.BlobGas())
				require.Equal(tx.BlobGasFeeCap(), evmTx.BlobGasFeeCap())
				require.Equal(tx.BlobHashes(), evmTx.BlobHashes())
				require.Equal(tx.BlobTxSidecar(), evmTx.BlobTxSidecar())

				switch tx.Type() {
				case types.LegacyTxType:
					require.Nil(evmTx.AccessList())
					fallthrough
				case types.AccessListTxType:
					require.Equal(evmTx.GasTipCap(), evmTx.GasPrice())
					require.Equal(evmTx.GasFeeCap(), evmTx.GasPrice())
					fallthrough
				case types.DynamicFeeTxType:
					require.Zero(evmTx.BlobGas())
					require.Nil(evmTx.BlobGasFeeCap())
					require.Nil(evmTx.BlobHashes())
					require.Nil(evmTx.BlobTxSidecar())
				}
				// verify signed tx hash and protected
				signer := MustNoErrorV(NewEthSigner(iotextypes.Encoding_ETHEREUM_EIP155, v.chainID))
				signedTx := MustNoErrorV(RawTxToSignedTx(evmTx, signer, sig))
				require.Equal(tx.Hash(), signedTx.Hash())
				require.Equal(tx.Protected(), signedTx.Protected())
			}
		})
	}
}

// TODO: test tx container
func TestEthTxDecodeVerifyV2(t *testing.T) {
	var (
		r        = require.New(t)
		sk, _    = crypto.HexStringToPrivateKey("708361c6460c93f027df0823eb440f3270ee2937944f2de933456a3900d6dd1a")
		nonce    = uint64(100)
		amount   = big.NewInt(1)
		to       = "io1c04r4jc8q6u57phxpla639u5h5v2t3s062hyxa"
		addrto   = common.BytesToAddress(MustNoErrorV(address.FromString(to)).Bytes())
		data     = []byte("any")
		gasLimit = uint64(21000)
		gasPrice = big.NewInt(101)
		chainID  = uint32(4689)
	)
	r.NotNil(sk)

	elpbuilder := (&EnvelopeBuilder{}).
		SetNonce(nonce).
		SetGasPrice(gasPrice).
		SetGasLimit(gasLimit).
		SetChainID(chainID)

	tests := []struct {
		name     string                                     // case name
		encoding iotextypes.Encoding                        // assign and check signer's type
		txto     string                                     // assertion of tx.To()
		txamount *big.Int                                   // assertion of tx.Value()
		txdata   []byte                                     // assertion of tx.Data()
		action   actionPayload                              // eth compatible action
		builder  func(*types.Transaction) (Envelope, error) // envelope builder
	}{
		{
			name:     "Transfer",
			encoding: iotextypes.Encoding_ETHEREUM_EIP155,
			txto:     to,
			txamount: amount,
			txdata:   data,
			action:   NewTransfer(amount, to, data),
			builder:  elpbuilder.BuildTransfer,
		},
		{
			name:     "Execution",
			encoding: iotextypes.Encoding_ETHEREUM_EIP155,
			txto:     to,
			txamount: amount,
			txdata:   data,
			action:   NewExecution(to, amount, data),
			builder:  elpbuilder.BuildExecution,
		},
		{
			name:     "ExecutionEmptyTo",
			encoding: iotextypes.Encoding_ETHEREUM_EIP155,
			txto:     "",
			txamount: amount,
			txdata:   data,
			action:   NewExecution("", amount, data),
			builder:  elpbuilder.BuildExecution,
		},
		{
			name:     "CreateStake",
			encoding: iotextypes.Encoding_ETHEREUM_EIP155,
			txto:     MustNoErrorV(address.FromBytes(address.StakingProtocolAddrHash[:])).String(),
			txamount: big.NewInt(0),
			txdata: append(
				_createStakeMethod.ID,
				MustNoErrorV(_createStakeMethod.Inputs.Pack("name", amount, uint32(86400), true, data))...,
			),
			action:  MustNoErrorV(NewCreateStake("name", amount.String(), 86400, true, data)),
			builder: elpbuilder.BuildStakingAction,
		},
		{
			name:     "DepositToStake",
			encoding: iotextypes.Encoding_ETHEREUM_EIP155,
			txto:     MustNoErrorV(address.FromBytes(address.StakingProtocolAddrHash[:])).String(),
			txamount: big.NewInt(0),
			txdata: append(
				_depositToStakeMethod.ID,
				MustNoErrorV(_depositToStakeMethod.Inputs.Pack(uint64(10), amount, data))...,
			),
			action:  MustNoErrorV(NewDepositToStake(10, amount.String(), data)),
			builder: elpbuilder.BuildStakingAction,
		},
		{
			name:     "ChangeStakeCandidate",
			encoding: iotextypes.Encoding_ETHEREUM_EIP155,
			txto:     MustNoErrorV(address.FromBytes(address.StakingProtocolAddrHash[:])).String(),
			txamount: big.NewInt(0),
			txdata: append(
				_changeCandidateMethod.ID,
				MustNoErrorV(_changeCandidateMethod.Inputs.Pack("name", uint64(11), data))...,
			),
			action:  NewChangeCandidate("name", 11, data),
			builder: elpbuilder.BuildStakingAction,
		},
		{
			name:     "UnStake",
			encoding: iotextypes.Encoding_ETHEREUM_EIP155,
			txto:     MustNoErrorV(address.FromBytes(address.StakingProtocolAddrHash[:])).String(),
			txamount: big.NewInt(0),
			txdata: append(
				_unstakeMethod.ID,
				MustNoErrorV(_unstakeMethod.Inputs.Pack(uint64(12), data))...,
			),
			action:  NewUnstake(12, data),
			builder: elpbuilder.BuildStakingAction,
		},
		{
			name:     "StakeWithdraw",
			encoding: iotextypes.Encoding_ETHEREUM_EIP155,
			txto:     MustNoErrorV(address.FromBytes(address.StakingProtocolAddrHash[:])).String(),
			txamount: big.NewInt(0),
			txdata: append(
				_withdrawStakeMethod.ID,
				MustNoErrorV(_withdrawStakeMethod.Inputs.Pack(uint64(13), data))...,
			),
			action:  NewWithdrawStake(13, data),
			builder: elpbuilder.BuildStakingAction,
		},
		{
			name:     "ReStake",
			encoding: iotextypes.Encoding_ETHEREUM_EIP155,
			txto:     MustNoErrorV(address.FromBytes(address.StakingProtocolAddrHash[:])).String(),
			txamount: big.NewInt(0),
			txdata: append(
				_restakeMethod.ID,
				MustNoErrorV(_restakeMethod.Inputs.Pack(uint64(14), uint32(7200), false, data))...,
			),
			action:  NewRestake(14, 7200, false, data),
			builder: elpbuilder.BuildStakingAction,
		},
		{
			name:     "TransferStakeOwnership",
			encoding: iotextypes.Encoding_ETHEREUM_EIP155,
			txto:     MustNoErrorV(address.FromBytes(address.StakingProtocolAddrHash[:])).String(),
			txamount: big.NewInt(0),
			txdata: append(
				_transferStakeMethod.ID,
				MustNoErrorV(_transferStakeMethod.Inputs.Pack(
					common.BytesToAddress(MustNoErrorV(address.FromString(to)).Bytes()),
					uint64(15), data),
				)...,
			),
			action:  MustNoErrorV(NewTransferStake(to, 15, data)),
			builder: elpbuilder.BuildStakingAction,
		},
		{
			name:     "RegisterStakeCandidate",
			encoding: iotextypes.Encoding_ETHEREUM_EIP155,
			txto:     MustNoErrorV(address.FromBytes(address.StakingProtocolAddrHash[:])).String(),
			txamount: big.NewInt(0),
			txdata: append(
				_candidateRegisterMethod.ID,
				MustNoErrorV(_candidateRegisterMethod.Inputs.Pack(
					"name", addrto, addrto, addrto, amount, uint32(6400), false, data,
				))...,
			),
			action:  MustNoErrorV(NewCandidateRegister("name", to, to, to, amount.String(), 6400, false, data)),
			builder: elpbuilder.BuildStakingAction,
		},
		{
			name:     "UpdateStakeCandidate",
			encoding: iotextypes.Encoding_ETHEREUM_EIP155,
			txto:     MustNoErrorV(address.FromBytes(address.StakingProtocolAddrHash[:])).String(),
			txamount: big.NewInt(0),
			txdata: append(
				_candidateUpdateMethod.ID,
				MustNoErrorV(_candidateUpdateMethod.Inputs.Pack("name", addrto, addrto))...,
			),
			action:  MustNoErrorV(NewCandidateUpdate("name", to, to)),
			builder: elpbuilder.BuildStakingAction,
		},
		{
			name:     "StakeCandidateActivate",
			encoding: iotextypes.Encoding_ETHEREUM_EIP155,
			txto:     MustNoErrorV(address.FromBytes(address.StakingProtocolAddrHash[:])).String(),
			txamount: big.NewInt(0),
			txdata: append(
				candidateActivateMethod.ID,
				MustNoErrorV(candidateActivateMethod.Inputs.Pack(uint64(16)))...,
			),
			action:  NewCandidateActivate(16),
			builder: elpbuilder.BuildStakingAction,
		},
		{
			name:     "StakeCandidateEndorsement",
			encoding: iotextypes.Encoding_ETHEREUM_EIP155,
			txto:     MustNoErrorV(address.FromBytes(address.StakingProtocolAddrHash[:])).String(),
			txamount: big.NewInt(0),
			txdata: append(
				candidateEndorsementLegacyMethod.ID,
				MustNoErrorV(candidateEndorsementLegacyMethod.Inputs.Pack(uint64(17), false))...,
			),
			action:  NewCandidateEndorsementLegacy(17, false),
			builder: elpbuilder.BuildStakingAction,
		},
		{
			name:     "StakeCandidateTransferOwnership",
			encoding: iotextypes.Encoding_ETHEREUM_EIP155,
			txto:     MustNoErrorV(address.FromBytes(address.StakingProtocolAddrHash[:])).String(),
			txamount: big.NewInt(0),
			txdata: append(
				_candidateTransferOwnershipMethod.ID,
				MustNoErrorV(_candidateTransferOwnershipMethod.Inputs.Pack(addrto, data))...,
			),
			action:  MustNoErrorV(NewCandidateTransferOwnership(to, data)),
			builder: elpbuilder.BuildStakingAction,
		},
		{
			name:     "ClaimFromRewardingFund",
			encoding: iotextypes.Encoding_ETHEREUM_EIP155,
			txto:     MustNoErrorV(address.FromBytes(address.RewardingProtocolAddrHash[:])).String(),
			txamount: big.NewInt(0),
			txdata: append(
				_claimRewardingMethodV2.ID,
				MustNoErrorV(_claimRewardingMethodV2.Inputs.Pack(amount, to, data))...,
			),
			action: &ClaimFromRewardingFund{
				amount:  amount,
				address: MustNoErrorV(address.FromString(to)),
				data:    data,
			},
			builder: elpbuilder.BuildRewardingAction,
		},
		{
			name:     "DepositToRewardingFund",
			encoding: iotextypes.Encoding_ETHEREUM_EIP155,
			txto:     MustNoErrorV(address.FromBytes(address.RewardingProtocolAddrHash[:])).String(),
			txamount: big.NewInt(0),
			txdata: append(
				_depositRewardMethod.ID,
				MustNoErrorV(_depositRewardMethod.Inputs.Pack(amount, data))...,
			),
			action: &DepositToRewardingFund{
				amount: amount,
				data:   data,
			},
			builder: elpbuilder.BuildRewardingAction,
		},
		{
			name:     "UnprotectedExecution",
			encoding: iotextypes.Encoding_ETHEREUM_UNPROTECTED,
			txto:     to,
			txamount: amount,
			txdata:   data,
			action:   NewExecution(to, amount, data),
			builder:  elpbuilder.BuildExecution,
		},
		{
			name:     "StakeMigrate",
			encoding: iotextypes.Encoding_ETHEREUM_EIP155,
			txto:     MustNoErrorV(address.FromBytes(address.StakingProtocolAddrHash[:])).String(),
			txamount: big.NewInt(0),
			txdata: append(
				migrateStakeMethod.ID,
				MustNoErrorV(migrateStakeMethod.Inputs.Pack(uint64(1)))...,
			),
			action:  NewMigrateStake(1),
			builder: elpbuilder.BuildStakingAction,
		},
		{
			name:     "CandidateEndorsementEndorse",
			encoding: iotextypes.Encoding_ETHEREUM_EIP155,
			txto:     MustNoErrorV(address.FromBytes(address.StakingProtocolAddrHash[:])).String(),
			txamount: big.NewInt(0),
			txdata: append(
				candidateEndorsementEndorseMethod.ID,
				MustNoErrorV(candidateEndorsementEndorseMethod.Inputs.Pack(uint64(17)))...,
			),
			action:  MustNoErrorV(NewCandidateEndorsement(17, CandidateEndorsementOpEndorse)),
			builder: elpbuilder.BuildStakingAction,
		},
		{
			name:     "CandidateEndorsementIntentToRevoke",
			encoding: iotextypes.Encoding_ETHEREUM_EIP155,
			txto:     MustNoErrorV(address.FromBytes(address.StakingProtocolAddrHash[:])).String(),
			txamount: big.NewInt(0),
			txdata: append(
				caniddateEndorsementIntentToRevokeMethod.ID,
				MustNoErrorV(caniddateEndorsementIntentToRevokeMethod.Inputs.Pack(uint64(17)))...,
			),
			action:  MustNoErrorV(NewCandidateEndorsement(17, CandidateEndorsementOpIntentToRevoke)),
			builder: elpbuilder.BuildStakingAction,
		},
		{
			name:     "CandidateEndorsementRevoke",
			encoding: iotextypes.Encoding_ETHEREUM_EIP155,
			txto:     MustNoErrorV(address.FromBytes(address.StakingProtocolAddrHash[:])).String(),
			txamount: big.NewInt(0),
			txdata: append(
				candidateEndorsementRevokeMethod.ID,
				MustNoErrorV(candidateEndorsementRevokeMethod.Inputs.Pack(uint64(17)))...,
			),
			action:  MustNoErrorV(NewCandidateEndorsement(17, CandidateEndorsementOpRevoke)),
			builder: elpbuilder.BuildStakingAction,
		},
	}

	verifytx := func(tx *types.Transaction, enc iotextypes.Encoding, amount *big.Int, data []byte, to string, sigv *big.Int) {
		r.Equal(tx.Nonce(), nonce)
		r.Equal(tx.Gas(), gasLimit)
		r.Equal(tx.GasPrice(), gasPrice)
		r.Equal(tx.Type(), uint8(types.LegacyTxType))
		r.Equal(tx.Value(), amount)
		r.Equal(tx.Data(), data)
		if to == "" {
			r.Nil(tx.To())
		} else {
			r.Equal(to, strings.ToLower(tx.To().String()))
		}
		if enc == iotextypes.Encoding_ETHEREUM_UNPROTECTED {
			r.False(tx.Protected())
			r.Zero(tx.ChainId().Uint64())
			r.True(sigv.Uint64() == 27 || sigv.Uint64() == 28)
		} else if enc == iotextypes.Encoding_ETHEREUM_EIP155 {
			r.True(tx.Protected())
			r.Equal(tx.ChainId().Uint64(), uint64(chainID))
			r.True(sigv.Uint64() > 28)
		}
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			toaddr := ""
			if tt.txto != "" {
				toaddr = "0x" + hex.EncodeToString(MustNoErrorV(address.FromString(tt.txto)).Bytes())
			}
			// build eth tx from test case
			var (
				elp       = elpbuilder.SetAction(tt.action).Build()
				tx        = MustNoErrorV(elp.ToEthTx(chainID, tt.encoding))
				signer    = MustNoErrorV(NewEthSigner(tt.encoding, chainID))
				signature = MustNoErrorV(sk.Sign(tx.Hash().Bytes()))
				builttx   = MustNoErrorV(RawTxToSignedTx(tx, signer, signature))
				raw       = MustNoErrorV(builttx.MarshalBinary())
			)

			sv, _, _ := builttx.RawSignatureValues()
			verifytx(builttx, tt.encoding, tt.txamount, tt.txdata, toaddr, sv)

			// decode eth tx from builttx hex raw
			decodedtx, err := DecodeEtherTx(hex.EncodeToString(raw))
			r.NoError(err)
			enc, sig, pk, err := ExtractTypeSigPubkey(decodedtx)
			r.NoError(err)
			r.Equal(enc, tt.encoding)
			r.Equal(sig[:64], signature[:64])
			sv, _, _ = decodedtx.RawSignatureValues()
			verifytx(decodedtx, tt.encoding, tt.txamount, tt.txdata, toaddr, sv)

			// build elp from eth tx by native converter
			elp, err = tt.builder(decodedtx)
			r.NoError(err)
			sealed, err := (&Deserializer{}).
				SetEvmNetworkID(chainID).
				ActionToSealedEnvelope(&iotextypes.Action{
					Core:         elp.Proto(),
					SenderPubKey: pk.Bytes(),
					Signature:    sig,
					Encoding:     tt.encoding,
				})
			r.NoError(err)
			convertedrawtx, err := sealed.ToEthTx()
			r.NoError(err)
			r.NoError(sealed.VerifySignature())
			convertedsignedtx, err := RawTxToSignedTx(convertedrawtx, signer, signature)
			r.NoError(err)
			r.Equal(convertedsignedtx.Hash(), decodedtx.Hash())
			sv, _, _ = convertedsignedtx.RawSignatureValues()
			verifytx(convertedsignedtx, tt.encoding, tt.txamount, tt.txdata, toaddr, sv)
		})
	}
}

func convertToNativeProto(tx *types.Transaction, actType string) (*iotextypes.ActionCore, error) {
	elpBuilder := (&EnvelopeBuilder{}).SetChainID(1)
	elpBuilder.SetGasLimit(tx.Gas()).SetGasPrice(tx.GasPrice()).SetNonce(tx.Nonce())
	var (
		elp Envelope
		err error
	)
	switch actType {
	case "transfer", "blobtx":
		elp, err = elpBuilder.BuildTransfer(tx)
	case "execution", "unprotected", "accesslist", "dynamicfee":
		elp, err = elpBuilder.BuildExecution(tx)
	case "stakeCreate", "stakeAddDeposit", "changeCandidate", "unstake", "withdrawStake", "restake",
		"transferStake", "candidateRegister", "candidateUpdate", "candidateActivate", "candidateEndorsement", "candidateTransferOwnership",
		"endorseCandidate", "intentToRevokeEndorsement", "revokeEndorsement", "migrateStake":
		elp, err = elpBuilder.BuildStakingAction(tx)
	case "rewardingClaim", "rewardingClaimWithAddress", "rewardingDeposit":
		elp, err = elpBuilder.BuildRewardingAction(tx)
	default:
		panic("unsupported")
	}
	if err != nil {
		return nil, err
	}
	return elp.Proto(), nil
}

func checkContract(to string, actType string) func(context.Context, *common.Address) (bool, bool, bool, error) {
	if to == "" {
		return func(context.Context, *common.Address) (bool, bool, bool, error) {
			return true, false, false, nil
		}
	}
	var (
		addr, _ = address.FromHex(to)
		ioAddr  = addr.String()
	)
	if ioAddr == address.StakingProtocolAddr {
		return func(context.Context, *common.Address) (bool, bool, bool, error) {
			return false, true, false, nil
		}
	}
	if ioAddr == address.RewardingProtocol {
		return func(context.Context, *common.Address) (bool, bool, bool, error) {
			return false, false, true, nil
		}
	}
	switch actType {
	case "transfer", "blobtx":
		return func(context.Context, *common.Address) (bool, bool, bool, error) {
			return false, false, false, nil
		}
	case "execution", "unprotected", "accesslist", "dynamicfee":
		return func(context.Context, *common.Address) (bool, bool, bool, error) {
			return true, false, false, nil
		}
	default:
		panic("unsupported")
	}
}

func TestIssue3944(t *testing.T) {
	r := require.New(t)
	// the sample tx below is the web3 format tx on the testnet:
	// https://testnet.iotexscan.io/tx/fcaf377ff3cc785d60c58de7e121d6a2e79e1c58c189ea8641f3ea61f7605285
	// or you can get it using ethclient code:
	// {
	//		cli, _ := ethclient.Dial("https://babel-api.testnet.iotex.io")
	// 		tx, _, err := cli.TransactionByHash(context.Background(), common.HexToHash("0xfcaf377ff3cc785d60c58de7e121d6a2e79e1c58c189ea8641f3ea61f7605285"))
	// }
	var (
		hash    = "0xfcaf377ff3cc785d60c58de7e121d6a2e79e1c58c189ea8641f3ea61f7605285"
		data, _ = hex.DecodeString("f3fef3a3000000000000000000000000fdff3eafde9a0cc42d18aab2a7454b1105f19edf00000000000000000000000000000000000000000000002086ac351052600000")
		to      = common.HexToAddress("0xd313b3131e238C635f2fE4a84EaDaD71b3ed25fa")
		sig, _  = hex.DecodeString("adff1da88c93f4e80c27bab0d613147fb7aeeed6e976231695de52cd9ac5aa8a3094e02759b838514f8376e05ceb266badc791ac2e7045ee7c15e58fc626980b1b")
	)
	tx := types.NewTx(&types.LegacyTx{
		Nonce:    83,
		To:       &to,
		Value:    new(big.Int),
		Gas:      39295,
		GasPrice: big.NewInt(1000000000000),
		Data:     data,
		V:        big.NewInt(int64(sig[64])),
		R:        new(big.Int).SetBytes(sig[:32]),
		S:        new(big.Int).SetBytes(sig[32:64]),
	})

	r.EqualValues(types.LegacyTxType, tx.Type())
	r.False(tx.Protected())
	v, q, s := tx.RawSignatureValues()
	r.Equal(sig[:32], q.Bytes())
	r.Equal(sig[32:64], s.Bytes())
	r.Equal("1b", v.Text(16))
	r.NotEqual(hash, tx.Hash().Hex()) // hash does not match with wrong V value in signature

	signer, err := NewEthSigner(iotextypes.Encoding_ETHEREUM_EIP155, 4690)
	r.NoError(err)
	tx1, err := RawTxToSignedTx(tx, signer, sig)
	r.NoError(err)
	r.True(tx1.Protected())
	r.EqualValues(4690, tx1.ChainId().Uint64())
	v, q, s = tx1.RawSignatureValues()
	r.Equal(sig[:32], q.Bytes())
	r.Equal(sig[32:64], s.Bytes())
	r.Equal("9415", v.String()) // this is the correct V value corresponding to chainID = 4690
	r.Equal(hash, tx1.Hash().Hex())
}

// generateRLPTestRaw generates RLP tx raw data for testing
func generateRLPTestRaw(sk *ecdsa.PrivateKey, test *rlpTest) string {
	var (
		tx types.TxData
		to *common.Address
	)
	if test.to != EmptyAddress {
		addr := common.HexToAddress(test.to)
		to = &addr
	}
	switch test.actType {
	case "transfer", "execution":
		fallthrough
	case "stakeCreate", "stakeAddDeposit", "unstake", "withdrawStake", "restake",
		"transferStake", "changeCandidate", "candidateRegister", "candidateUpdate",
		"candidateActivate", "candidateTransferOwnership", "migrateStake", "endorseCandidate",
		"candidateEndorsement", "intentToRevokeEndorsement", "revokeEndorsement":
		fallthrough
	case "rewardingClaim", "rewardingClaimWithAddress", "rewardingDeposit":
		tx = &types.LegacyTx{
			Nonce:    test.nonce,
			GasPrice: MustBeTrueV(new(big.Int).SetString(test.price, 10)),
			Gas:      test.limit,
			To:       to,
			Value:    MustBeTrueV(new(big.Int).SetString(test.amount, 10)),
			Data:     test.data,
		}
	case "unprotected":
		return deterministicDeploymentTx
	case "accesslist":
		tx = &types.AccessListTx{
			Nonce:      test.nonce,
			GasPrice:   MustBeTrueV(new(big.Int).SetString(test.price, 10)),
			Gas:        test.limit,
			To:         to,
			Value:      MustBeTrueV(new(big.Int).SetString(test.amount, 10)),
			Data:       test.data,
			AccessList: createTestACL(),
		}
	case "dynamicfee":
		tip := MustBeTrueV(new(big.Int).SetString(test.price, 10))
		tx = &types.DynamicFeeTx{
			Nonce:     test.nonce,
			GasTipCap: tip,
			GasFeeCap: new(big.Int).Lsh(tip, 1),
			Gas:       test.limit,
			To:        to,
			Value:     MustBeTrueV(new(big.Int).SetString(test.amount, 10)),
			Data:      test.data,
			AccessList: types.AccessList{
				{Address: common.Address{}, StorageKeys: nil},
				{Address: _c1, StorageKeys: []common.Hash{_k1, {}, _k3}},
				{Address: _c2, StorageKeys: []common.Hash{_k2, _k3, _k4, _k1}},
			},
		}
	case "blobtx":
		price := new(uint256.Int)
		if err := price.SetFromDecimal(test.price); err != nil {
			panic(err.Error())
		}
		value := new(uint256.Int)
		if err := value.SetFromDecimal(test.amount); err != nil {
			panic(err.Error())
		}
		blob := createTestBlobTxData()
		tx = &types.BlobTx{
			Nonce:     test.nonce,
			GasTipCap: price,
			GasFeeCap: new(uint256.Int).Lsh(price, 1),
			Gas:       test.limit,
			To:        *to,
			Value:     value,
			Data:      test.data,
			AccessList: types.AccessList{
				{Address: common.Address{}, StorageKeys: nil},
				{Address: _c1, StorageKeys: []common.Hash{_k1, {}, _k3}},
				{Address: _c2, StorageKeys: []common.Hash{_k2, _k3, _k4, _k1}},
			},
			BlobFeeCap: blob.gasFeeCap(),
			BlobHashes: blob.hashes(),
			Sidecar:    blob.sidecar,
		}
	default:
		panic("not supported")
	}
	signer := types.NewCancunSigner(big.NewInt(int64(test.chainID)))
	signedtx := types.MustSignNewTx(sk, signer, tx)
	return hex.EncodeToString(MustNoErrorV(signedtx.MarshalBinary()))
}
