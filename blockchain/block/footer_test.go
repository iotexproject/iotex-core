// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package block

import (
	"testing"
	"time"

	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/endorsement"
	"github.com/iotexproject/iotex-core/test/identityset"
)

func TestConvertToBlockFooterPb(t *testing.T) {
	require := require.New(t)
	footer := &Footer{nil, time.Now()}
	blockFooter, err := footer.ConvertToBlockFooterPb()
	require.NoError(err)
	require.NotNil(blockFooter)
	require.Equal(0, len(blockFooter.Endorsements))

	footer = makeFooter()
	blockFooter, err = footer.ConvertToBlockFooterPb()
	require.NoError(err)
	require.NotNil(blockFooter)
	require.Equal(1, len(blockFooter.Endorsements))
}

func TestConvertFromBlockFooterPb(t *testing.T) {
	require := require.New(t)
	ts := &timestamp.Timestamp{Seconds: 10, Nanos: 10}
	footerPb := &iotextypes.BlockFooter{nil, ts, struct{}{}, nil, 0}
	footer := &Footer{}
	require.NoError(footer.ConvertFromBlockFooterPb(footerPb))
}

func TestSerDesFooter(t *testing.T) {
	require := require.New(t)
	footer := &Footer{nil, time.Now()}
	ser, err := footer.Serialize()
	require.NoError(err)
	require.NoError(footer.Deserialize(ser))
	require.Equal(0, len(footer.endorsements))

	footer = makeFooter()
	require.NoError(err)
	ser, err = footer.Serialize()
	require.NoError(err)
	require.NoError(footer.Deserialize(ser))
	require.Equal(1, len(footer.endorsements))
}

func makeFooter() (f *Footer) {
	endors := make([]*endorsement.Endorsement, 0)
	endor := endorsement.NewEndorsement(time.Now(), identityset.PrivateKey(27).PublicKey(), nil)
	endors = append(endors, endor)
	f = &Footer{endors, time.Now()}
	return
}
