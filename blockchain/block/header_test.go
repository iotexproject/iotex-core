// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package block

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/stretchr/testify/require"
)

func TestSerDesHeadrer(t *testing.T) {
	require := require.New(t)
	for _, hasBlob := range []bool{
		false, true,
	} {
		h := getHeader(hasBlob)
		pb := h.BlockHeaderCoreProto()
		if hasBlob {
			require.EqualValues(189000, pb.GasUsed)
			require.Equal("0f4240", hex.EncodeToString(pb.BaseFee))
			require.True(isEqual("26e0fa68a06833a42c7e008df9147638da6011e38c8e8437f334c31d62c7ce2f", h.HashHeaderCore()))
		} else {
			require.Zero(pb.GasUsed)
			require.Nil(pb.BaseFee)
			require.True(isEqual("13bf151a1a8ac8d45c636308cd3dd516e2811c9a7985a16c13c6a913a6faed7b", h.HashHeaderCore()))
		}
		ser, err := h.Serialize()
		require.NoError(err)
		require.NotNil(ser)
		header := &Header{}
		require.NoError(header.Deserialize(ser))
		require.Equal(uint32(1), header.Version())
		require.Equal(uint64(2), header.Height())
		ti, err := time.Parse("2006-Jan-02", "2019-Feb-03")
		require.NoError(err)
		require.Equal(ti, header.Timestamp())
		expected := "c5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470"
		require.True(isEqual(expected, header.PrevHash()))
		require.True(isEqual(expected, header.TxRoot()))
		require.True(isEqual(expected, header.DeltaStateDigest()))
		require.Equal("04755ce6d8903f6b3793bddb4ea5d3589d637de2d209ae0ea930815c82db564ee8cc448886f639e8a0c7e94e99a5c1335b583c0bc76ef30dd6a1038ed9da8daf33", header.PublicKey().HexString())
		require.True(isEqual(expected, header.ReceiptRoot()))
		require.Equal("io1mflp9m6hcgm2qcghchsdqj3z3eccrnekx9p0ms", header.ProducerAddress())
		if hasBlob {
			require.EqualValues(189000, header.gasUsed)
			require.Equal("1000000", header.baseFee.String())
			require.True(isEqual("d8e43d580c053c5f26f09a9252725cb0faa911cd7a6954e6ff2e28d336baa379", header.HashBlock()))
		} else {
			require.Zero(header.gasUsed)
			require.Nil(header.baseFee)
			require.True(isEqual("39f9a57253c8396601394ca504ff0cd648adefbd1d0728e9e77fd211e34c5258", header.HashBlock()))
		}
	}
}
func getHeader(hasBlob bool) *Header {
	ti, err := time.Parse("2006-Jan-02", "2019-Feb-03")
	if err != nil {
		return nil
	}
	h := Header{
		version:          1,
		height:           2,
		timestamp:        ti,
		prevBlockHash:    hash.Hash256b([]byte("")),
		txRoot:           hash.Hash256b([]byte("")),
		deltaStateDigest: hash.Hash256b([]byte("")),
		receiptRoot:      hash.Hash256b([]byte("")),
		blockSig:         nil,
		pubkey:           identityset.PrivateKey(27).PublicKey(),
	}
	if hasBlob {
		h.gasUsed = 189000
		h.baseFee = big.NewInt(1000000)
	}
	return &h
}
func isEqual(expected string, hash hash.Hash256) bool {
	h := fmt.Sprintf("%x", hash[:])
	return strings.EqualFold(expected, h)
}
