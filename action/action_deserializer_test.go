// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/big"
	"testing"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil"
)

func TestActionDeserializer(t *testing.T) {
	r := require.New(t)
	for _, v := range []struct {
		id          uint32
		hash, hash2 string
	}{
		{0, "322884fb04663019be6fb461d9453827487eafdd57b4de3bd89a7d77c9bf8395", "0562e100b057804ee3cb4fa906a897852aa8075013a02ef1e229360f1e5ee339"},
		{1, "80af7840d73772d3022d8bdc46278fb755352e5e9d5f2a1f12ee7ec4f1ea98e9", "405343d671c395d77835b8857cc25317e3bf02680f8c875a4fe12087b0446184"},
	} {
		se, err := createSealedEnvelope(v.id)
		r.NoError(err)
		rHash, err := se.Hash()
		r.NoError(err)
		r.Equal(v.hash, hex.EncodeToString(rHash[:]))
		r.Equal(v.id, se.ChainID())
		r.Equal(_publicKey, se.SrcPubkey().HexString())
		r.Equal(_signByte, se.Signature())
		r.Zero(se.Encoding())

		// use valid signature and reset se.Hash
		se.signature = _validSig
		se.hash = hash.ZeroHash256
		rHash, err = se.Hash()
		r.NoError(err)
		r.Equal(v.hash2, hex.EncodeToString(rHash[:]))
		se1, err := (&Deserializer{}).ActionToSealedEnvelope(se.Proto())
		r.NoError(err)
		rHash, err = se1.Hash()
		r.NoError(err)
		r.Equal(v.hash2, hex.EncodeToString(rHash[:]))
		r.Equal(se, se1)
	}
}

func TestProtoWithChainID(t *testing.T) {
	r := require.New(t)
	txID0, _ := hex.DecodeString("0a10080118a08d062202313062040a023130124104dc4c548c3a478278a6a09ffa8b5c4b384368e49654b35a6961ee8288fc889cdc39e9f8194e41abdbfac248ef9dc3f37b131a36ee2c052d974c21c1d2cd56730b1a4161e219c2c5d5987f8a9efa33e8df0cde9d5541689fff05784cdc24f12e9d9ee8283a5aa720f494b949535b7969c07633dfb68c4ef9359eb16edb9abc6ebfadc801")
	txID1, _ := hex.DecodeString("0a12080118a08d0622023130280162040a023130124104dc4c548c3a478278a6a09ffa8b5c4b384368e49654b35a6961ee8288fc889cdc39e9f8194e41abdbfac248ef9dc3f37b131a36ee2c052d974c21c1d2cd56730b1a41328c6912fa0e36414c38089c03e2fa8c88bba82ccc4ce5fb8ac4ef9f529dfce249a5b2f93a45b818e7f468a742b4e87be3b8077f95d1b3c49e9165b971848ead01")
	var ad Deserializer
	for _, v := range []struct {
		data    []byte
		chainID uint32
		err     error
	}{
		{txID0, 0, nil},
		{txID1, 1, nil},
	} {
		tx := iotextypes.Action{}
		r.NoError(proto.Unmarshal(v.data, &tx))
		selp, err := ad.ActionToSealedEnvelope(&tx)
		r.NoError(err)
		r.Equal(v.chainID, selp.Envelope.ChainID())
		_, err = selp.Hash()
		r.NoError(err)
		r.Equal(v.err, selp.VerifySignature())
	}
}

func TestDataContanetation(t *testing.T) {
	require := require.New(t)

	// raw data
	tx1, _ := SignedTransfer(identityset.Address(28).String(),
		identityset.PrivateKey(28), 3, big.NewInt(10), []byte{}, testutil.TestGasLimit,
		big.NewInt(testutil.TestGasPriceInt64))
	txHash1, _ := tx1.Hash()
	serilizedTx1, err := proto.Marshal(tx1.Proto())
	require.NoError(err)
	tx2, _ := SignedExecution(identityset.Address(29).String(),
		identityset.PrivateKey(29), 1, big.NewInt(0), testutil.TestGasLimit,
		big.NewInt(testutil.TestGasPriceInt64), []byte{})
	txHash2, _ := tx2.Hash()
	serilizedTx2, err := proto.Marshal(tx2.Proto())
	require.NoError(err)

	t.Run("tx1", func(t *testing.T) {
		act1 := &iotextypes.Action{}
		err := proto.Unmarshal(serilizedTx1, act1)
		require.NoError(err)
		se1, err := (&Deserializer{}).ActionToSealedEnvelope(act1)
		require.NoError(err)
		actHash1, err := se1.Hash()
		require.NoError(err)
		require.Equal(txHash1, actHash1)

		act2 := &iotextypes.Action{}
		err = proto.Unmarshal(serilizedTx2, act2)
		require.NoError(err)
	})

	t.Run("tx2", func(t *testing.T) {
		act2 := &iotextypes.Action{}
		err = proto.Unmarshal(serilizedTx2, act2)
		require.NoError(err)
		se2, err := (&Deserializer{}).ActionToSealedEnvelope(act2)
		require.NoError(err)
		actHash2, err := se2.Hash()
		require.NoError(err)
		require.Equal(txHash2, actHash2)
	})

	t.Run("tx1+tx2", func(t *testing.T) {
		acts := &iotextypes.Actions{}
		err = proto.Unmarshal(append(serilizedTx1, serilizedTx2...), acts)
		require.NoError(err)
		require.Equal(2, len(acts.Actions))
		_, err := (&Deserializer{}).ActionToSealedEnvelope(acts.Actions[0])
		require.Error(err)
	})

	rawActs1 := &iotextypes.Actions{
		Actions: []*iotextypes.Action{tx1.Proto()},
	}
	serilizedActs1, err := proto.Marshal(rawActs1)
	require.NoError(err)
	rawActs2 := &iotextypes.Actions{
		Actions: []*iotextypes.Action{tx2.Proto()},
	}
	serilizedActs2, err := proto.Marshal(rawActs2)
	require.NoError(err)

	t.Run("tx1", func(t *testing.T) {
		acts := &iotextypes.Actions{}
		err = proto.Unmarshal(serilizedActs1, acts)
		require.NoError(err)
		require.Equal(1, len(acts.Actions))
		se, err := (&Deserializer{}).ActionToSealedEnvelope(acts.Actions[0])
		require.NoError(err)
		actHash, err := se.Hash()
		require.NoError(err)
		require.Equal(txHash1, actHash)
	})

	t.Run("tx2", func(t *testing.T) {
		acts := &iotextypes.Actions{}
		err = proto.Unmarshal(serilizedActs2, acts)
		require.NoError(err)
		require.Equal(1, len(acts.Actions))
		se, err := (&Deserializer{}).ActionToSealedEnvelope(acts.Actions[0])
		require.NoError(err)
		actHash, err := se.Hash()
		require.NoError(err)
		require.Equal(txHash2, actHash)
	})

	t.Run("tx1+tx2+tx1", func(t *testing.T) {
		acts := &iotextypes.Actions{}
		data := append(serilizedActs1, serilizedActs2...)
		data = append(data, serilizedActs1...)
		fmt.Println(binary.Size(data))
		err = proto.Unmarshal(data, acts)
		require.NoError(err)
		require.Equal(3, len(acts.Actions))
		se, err := (&Deserializer{}).ActionToSealedEnvelope(acts.Actions[0])
		require.NoError(err)
		actHash, err := se.Hash()
		require.NoError(err)
		require.Equal(txHash1, actHash)

		se, err = (&Deserializer{}).ActionToSealedEnvelope(acts.Actions[1])
		require.NoError(err)
		actHash, err = se.Hash()
		require.NoError(err)
		require.Equal(txHash2, actHash)

		se, err = (&Deserializer{}).ActionToSealedEnvelope(acts.Actions[2])
		require.NoError(err)
		actHash, err = se.Hash()
		require.NoError(err)
		require.Equal(txHash1, actHash)
	})
}
