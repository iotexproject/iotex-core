// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"encoding/hex"
	"testing"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestActionDeserializer(t *testing.T) {
	r := require.New(t)
	for _, v := range []struct {
		id   uint32
		hash string
	}{
		{0, "322884fb04663019be6fb461d9453827487eafdd57b4de3bd89a7d77c9bf8395"},
		{1, "80af7840d73772d3022d8bdc46278fb755352e5e9d5f2a1f12ee7ec4f1ea98e9"},
	} {
		se, err := createSealedEnvelope(v.id)
		r.NoError(err)

		r.Equal(v.id, se.ChainID())
		r.Equal(_publicKey, se.SrcPubkey().HexString())
		r.Equal(_signByte, se.Signature())
		r.Zero(se.Encoding())

		hashVal, hashErr := se.Hash()
		_ = se.SenderAddress()
		r.Equal(hex.EncodeToString(hashVal[:]), v.hash)
		r.NoError(hashErr)

		se, err = createSealedEnvelope(v.id)
		r.NoError(err)

		se.signature = _validSig
		validated, err := (&Deserializer{}).ActionToSealedEnvelope(se.Proto())

		_, err = se.Hash()
		r.NoError(err)
		_ = se.SenderAddress()
		_, err = validated.Hash()
		r.NoError(err)
		_ = validated.SenderAddress()
		r.Equal(validated, se)
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
