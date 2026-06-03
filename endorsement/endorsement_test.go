// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package endorsement

import (
	"bytes"
	"testing"
	"time"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

func TestEndorsement_LoadProto_Nil(t *testing.T) {
	r := require.New(t)
	en := &Endorsement{}
	r.NotPanics(func() {
		err := en.LoadProto(nil)
		r.Error(err)
	})
}

func TestEndorsement_LoadProto_RoundTrip(t *testing.T) {
	r := require.New(t)
	priKey := identityset.PrivateKey(0)
	sig := []byte("signature")
	now := time.Now()

	original := NewEndorsement(now, priKey.PublicKey(), sig)
	pb := original.Proto()

	loaded := &Endorsement{}
	r.NoError(loaded.LoadProto(pb))
	r.True(now.Equal(loaded.Timestamp()))
	r.Equal(priKey.PublicKey().HexString(), loaded.Endorser().HexString())
	r.Equal(0, bytes.Compare(sig, loaded.Signature()))
}

func TestEndorsement_LoadProto_InvalidEndorser(t *testing.T) {
	r := require.New(t)
	en := &Endorsement{}
	err := en.LoadProto(&iotextypes.Endorsement{
		// Timestamp left zero is valid (encodes epoch); Endorser bytes are garbage.
		Endorser: []byte{0x01, 0x02, 0x03},
	})
	r.Error(err)
}
