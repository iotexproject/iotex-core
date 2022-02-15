package action

import (
	"bytes"
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/config"
)

func TestGenerateRlp(t *testing.T) {
	require := require.New(t)

	ab := AbstractAction{
		version:  1,
		nonce:    2,
		gasLimit: 1000,
		gasPrice: new(big.Int),
	}
	rlpTsf := &Transfer{
		AbstractAction: ab,
		recipient:      "io1x9qa70ewgs24xwak66lz5dgm9ku7ap80vw3071",
	}
	rlpTsf1 := &Transfer{
		AbstractAction: ab,
		amount:         big.NewInt(100),
		recipient:      "",
	}
	hT1, _ := hex.DecodeString("87e39e819193ae46472eb1320739b34c4c3b38ea321c7cc503432bdcfd0cbf15")
	rlpTsf2 := &Transfer{
		AbstractAction: ab,
		recipient:      "io1x9qa70ewgs24xwak66lz5dgm9ku7ap80vw3070",
	}
	hT2, _ := hex.DecodeString("eaaf38a552809a9bdb1509c8093bd2c74eb07baff862dae692c1d2b865478b14")
	rlpExec := &Execution{
		AbstractAction: ab,
		amount:         big.NewInt(100),
		data:           signByte,
	}
	hE1, _ := hex.DecodeString("fcdd0c3d07f438d6e67ea852b40e5dc256d75f5e1fa9ac3ca96030efeb634150")
	rlpExec1 := &Execution{
		AbstractAction: ab,
		contract:       "io1x9qa70ewgs24xwak66lz5dgm9ku7ap80vw3070",
		amount:         big.NewInt(100),
		data:           signByte,
	}
	hE2, _ := hex.DecodeString("fee3db88ee7d7defa9eded672d08fc8641f760f3a11d404a53276ad6f412b8a5")
	rlpTests := []struct {
		act  RlpTransaction
		sig  []byte
		err  string
		hash hash.Hash256
	}{
		{nil, validSig, "nil action to generate RLP tx", hash.ZeroHash256},
		{rlpTsf, validSig, "invalid recipient address", hash.ZeroHash256},
		{rlpTsf1, signByte, "invalid signature length =", hash.ZeroHash256},
		{rlpTsf1, validSig, "", hash.BytesToHash256(hT1)},
		{rlpTsf2, validSig, "", hash.BytesToHash256(hT2)},
		{rlpExec, validSig, "", hash.BytesToHash256(hE1)},
		{rlpExec1, validSig, "", hash.BytesToHash256(hE2)},
	}

	for _, v := range rlpTests {
		_, err := generateRlpTx(v.act)
		if err != nil {
			require.Contains(err.Error(), v.err)
		}
		h, err := rlpSignedHash(v.act, 4689, v.sig)
		if err != nil {
			require.Contains(err.Error(), v.err)
		}
		require.Equal(v.hash, h)
	}
}

func TestRlpDecodeVerify(t *testing.T) {
	// register the extern chain ID
	config.SetEVMNetworkID(config.Default.Chain.EVMNetworkID)

	require := require.New(t)

	rlpTests := []struct {
		actType string
		raw     string
		nonce   uint64
		limit   uint64
		price   string
		amount  string
		to      string

		dataLen int
		hash    string
		pubkey  string
		pkhash  string
	}{
		{
			"transfer",
			"f86e8085e8d4a51000825208943141df3f2e4415533bb6d6be2a351b2db9ee84ef88016345785d8a0000808224c6a0204d25fc0d7d8b3fdf162c6ee820f888f5533b1c382d79d5cbc8ec1d9091a9a8a016f1a58d7e0d0fd24be800f64a2d6433c5fcb31e3fc7562b7fbe62bc382a95bb",
			0,
			21000,
			"1000000000000",
			"100000000000000000",
			"io1x9qa70ewgs24xwak66lz5dgm9ku7ap80vw3070",

			0,
			"eead45fe6b510db9ed6dce9187280791c04bbaadd90c54a7f4b1f75ced382ff1",
			"041ba784140be115e8fa8698933e9318558a895c75c7943100f0677e4d84ff2763ff68720a0d22c12d093a2d692d1e8292c3b7672fccf3b3db46a6e0bdad93be17",
			"87eea07540789af85b64947aea21a3f00400b597",
		},
		{
			"execution",
			"f8ab0d85e8d4a5100082520894ac7ac39de679b19aae042c0ce19facb86e0a411780b844a9059cbb0000000000000000000000003141df3f2e4415533bb6d6be2a351b2db9ee84ef000000000000000000000000000000000000000000000000000000003b9aca008224c5a0fac4e25db03c99fec618b74a962d322a334234696eb62c7e5b9889132ff4f4d7a02c88e451572ca36b6f690ce23ff9d6695dd71e888521fa706a8fc8c279099a61",
			13,
			21000,
			"1000000000000",
			"0",
			"io143av880x0xce4tsy9sxwr8avhphq5sghum77ct",
			68,
			"7467dd6ccd4f3d7b6dc0002b26a45ad0b75a1793da4e3557cf6ff2582cbe25c9",
			"041ba784140be115e8fa8698933e9318558a895c75c7943100f0677e4d84ff2763ff68720a0d22c12d093a2d692d1e8292c3b7672fccf3b3db46a6e0bdad93be17",
			"87eea07540789af85b64947aea21a3f00400b597",
		},
		{
			"execution",
			"f9024f2e830f42408381b3208080b901fc608060405234801561001057600080fd5b50336000806101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555061019c806100606000396000f3fe608060405234801561001057600080fd5b50600436106100415760003560e01c8063445df0ac146100465780638da5cb5b14610064578063fdacd576146100ae575b600080fd5b61004e6100dc565b6040518082815260200191505060405180910390f35b61006c6100e2565b604051808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200191505060405180910390f35b6100da600480360360208110156100c457600080fd5b8101908080359060200190929190505050610107565b005b60015481565b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1681565b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff16141561016457806001819055505b5056fea265627a7a72315820e54fe55a78b9d8bec22b4d3e6b94b7e59799daee3940423eb1aa30fe643eeb9a64736f6c634300051000328224c5a0439310c2d5509fc42486171b910cf8107542c86e23202a3a8ba43129cabcdbfea038966d36b41916f619c64bdc8c3ddcb021b35ea95d44875eb8201e9422fd98f0",
			46,
			8500000,
			"1000000",
			"0",
			EmptyAddress,
			508,
			"b676128dae841742e3ab6e518acb30badc6b26230fe870821d1de08c85823067",
			"049c6567f527f8fc98c0875d3d80097fcb4d5b7bfe037fc9dd5dbeaf563d58d7ff17a4f2b85df9734ecdb276622738e28f0b7cf224909ab7b128c5ca748729b0d2",
			"1904bfcb93edc9bf961eead2e5c0de81dcc1d37d",
		},
		{
			"stakeCreate",
			"f901cb02830f4240830186a094000000000000007374616b696e6743726561746580b90164a3d374c400000000000000000000000000000000000000000000000000000000000000a000000000000000000000000000000000000000000000000000000000000000640000000000000000000000000000000000000000000000000000000000000064000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000e00000000000000000000000000000000000000000000000000000000000000004746573740000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000030000000000000000000000000000000000000000000000000000000000000001000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000038224c5a050c4d2d0db373687ba97b2081c5466518d5676b6fbb9eab117db30d0fc419c0ba06b639f6772778568cb00ecce6b97f7d50c0393010543f83daf02ac4dcde4db29",
			2,
			100000,
			"1000000",
			"0",
			"io1qqqqqqqqqqq8xarpdd5kue6rwfjkzar9k0wk6t",
			356,
			"773d70ddbe616cbe9343655baf06006213a2e31297cd11eccec4de3b7e35e81a",
			"049c3a34c3f2053a769ed18f9266bc88e2404f3fc3c21fef75b561251d0af9898aa62e3fd0bfd5d4b0ac55cd912e200018fb8a396d29bf226962a082e8eec3cd20",
			"64b690bd2db36de68c003692fe8c24b5dcf9e320",
		},
	}

	for _, v := range rlpTests {
		encoded, err := hex.DecodeString(v.raw)
		require.NoError(err)

		// decode received RLP tx
		tx := types.Transaction{}
		require.NoError(rlp.DecodeBytes(encoded, &tx))

		// extract signature and recover pubkey
		w, r, s := tx.RawSignatureValues()
		recID := uint32(w.Int64()) - 2*config.EVMNetworkID() - 8
		sig := make([]byte, 64, 65)
		rSize := len(r.Bytes())
		copy(sig[32-rSize:32], r.Bytes())
		sSize := len(s.Bytes())
		copy(sig[64-sSize:], s.Bytes())
		sig = append(sig, byte(recID))

		// recover public key
		rawHash := types.NewEIP155Signer(big.NewInt(int64(config.EVMNetworkID()))).Hash(&tx)
		pubkey, err := crypto.RecoverPubkey(rawHash[:], sig)
		require.NoError(err)
		require.Equal(v.pubkey, pubkey.HexString())
		require.Equal(v.pkhash, hex.EncodeToString(pubkey.Hash()))

		// convert to our Execution
		pb := &iotextypes.Action{
			Encoding: iotextypes.Encoding_ETHEREUM_RLP,
		}
		pb.Core = convertToNativeProto(&tx, v.actType)
		pb.SenderPubKey = pubkey.Bytes()
		pb.Signature = sig

		// send on wire
		bs, err := proto.Marshal(pb)
		require.NoError(err)

		// receive from API
		proto.Unmarshal(bs, pb)
		selp := SealedEnvelope{}
		require.NoError(selp.LoadProto(pb))
		rlpTx, err := ToRLP(selp.Action())
		require.NoError(err)

		// verify against original tx
		require.Equal(v.nonce, rlpTx.Nonce())
		require.Equal(v.price, rlpTx.GasPrice().String())
		require.Equal(v.limit, rlpTx.GasLimit())
		require.Equal(v.to, rlpTx.Recipient())
		require.Equal(v.amount, rlpTx.Amount().String())
		require.Equal(v.dataLen, len(rlpTx.Payload()))
		h, err := selp.Hash()
		require.NoError(err)
		require.Equal(v.hash, hex.EncodeToString(h[:]))
		require.Equal(pubkey, selp.SrcPubkey())
		require.True(bytes.Equal(sig, selp.signature))
		raw, err := selp.envelopeHash()
		require.NoError(err)
		require.True(bytes.Equal(rawHash[:], raw[:]))
		require.NotEqual(raw, h)
		require.NoError(Verify(selp))
	}
}

func convertToNativeProto(tx *types.Transaction, actType string) *iotextypes.ActionCore {
	pb := iotextypes.ActionCore{
		Version:  1,
		Nonce:    tx.Nonce(),
		GasLimit: tx.Gas(),
		GasPrice: tx.GasPrice().String(),
	}
	ab := AbstractAction{
		nonce:    tx.Nonce(),
		gasLimit: tx.Gas(),
		gasPrice: tx.GasPrice(),
	}
	switch actType {
	case "transfer":
		tsf := &Transfer{}
		tsf.amount = tx.Value()
		ioAddr, _ := address.FromBytes(tx.To().Bytes())
		tsf.recipient = ioAddr.String()
		tsf.payload = tx.Data()
		tsf.AbstractAction = ab
		pb.Action = &iotextypes.ActionCore_Transfer{
			Transfer: tsf.Proto(),
		}
	case "execution":
		ex := &Execution{}
		ex.amount = tx.Value()
		if tx.To() != nil {
			ioAddr, _ := address.FromBytes(tx.To().Bytes())
			ex.contract = ioAddr.String()
		}
		ex.data = tx.Data()
		ex.AbstractAction = ab
		pb.Action = &iotextypes.ActionCore_Execution{
			Execution: ex.Proto(),
		}
	case "stakeCreate":
		act, _ := NewCreateStakeFromABIBinary(tx.Data())
		act.AbstractAction = ab
		pb.Action = &iotextypes.ActionCore_StakeCreate{
			StakeCreate: act.Proto(),
		}
	}
	return &pb
}
