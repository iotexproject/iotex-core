// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestActionDeserializer(t *testing.T) {
	r := require.New(t)
	se, err := createSealedEnvelope()
	r.NoError(err)
	rHash, err := se.Hash()
	r.NoError(err)
	r.Equal("322884fb04663019be6fb461d9453827487eafdd57b4de3bd89a7d77c9bf8395", hex.EncodeToString(rHash[:]))
	r.Equal(_publicKey, se.SrcPubkey().HexString())
	r.Equal(_signByte, se.Signature())
	r.Zero(se.Encoding())

	se.signature = _validSig
	se1, err := (&Deserializer{}).ActionToSealedEnvelope(se.Proto())
	r.NoError(err)
	r.Equal(se, se1)
}
