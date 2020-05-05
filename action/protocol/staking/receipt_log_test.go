// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"
	"testing"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/test/identityset"
)

func TestReceiptLog(t *testing.T) {
	r := require.New(t)

	h := hash.Hash160b([]byte(protocolID))
	addr, _ := address.FromBytes(h[:])
	addrString := addr.String()
	cand := identityset.Address(5)
	voter := identityset.Address(11)
	index := byteutil.Uint64ToBytesBigEndian(1)
	b1 := cand.Bytes()
	//b2 := voter.Bytes()

	ctx := protocol.WithActionCtx(context.Background(), protocol.ActionCtx{
		ActionHash: hash.Hash256b([]byte("test-action")),
	})
	ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
		BlockHeight: 6,
	})

	logTests := []struct {
		addr   string
		name   string
		topics [][]byte
		cand   address.Address
		voter  address.Address
		data   []byte
	}{
		{
			addrString,
			HandleCreateStake,
			[][]byte{index, b1},
			cand,
			voter,
			index,
		},
		{
			addrString,
			HandleUnstake,
			[][]byte{index, b1},
			nil,
			voter,
			nil,
		},
		{
			addrString,
			HandleWithdrawStake,
			[][]byte{index, b1},
			nil,
			voter,
			nil,
		},
		{
			addrString,
			HandleChangeCandidate,
			[][]byte{index, b1},
			cand,
			voter,
			nil,
		},
		{
			addrString,
			HandleTransferStake,
			[][]byte{index, b1},
			nil,
			voter,
			nil,
		},
		{
			addrString,
			HandleDepositToStake,
			[][]byte{index, b1},
			nil,
			voter,
			nil,
		},
		{
			addrString,
			HandleRestake,
			[][]byte{index, b1},
			nil,
			voter,
			nil,
		},
		{
			addrString,
			HandleCandidateRegister,
			[][]byte{index, b1},
			cand,
			voter,
			index,
		},
		{
			addrString,
			HandleCandidateUpdate,
			[][]byte{index, b1},
			nil,
			voter,
			nil,
		},
	}

	for _, v := range logTests {
		log := newReceiptLog(v.addr, v.name, false)
		log.AddTopics(v.topics...)
		log.AddAddress(v.cand)
		log.AddAddress(v.voter)
		log.SetData(v.data)
		r.Nil(log.Build(ctx))
		log.SetSuccess()
		r.EqualValues(log.Build(ctx), createLog(ctx, v.name, v.cand, v.voter, v.data))
	}
}

// only used for verify preFairbank receipt generation
func createLog(
	ctx context.Context,
	handlerName string,
	candidateAddr,
	voterAddr address.Address,
	data []byte,
) *action.Log {
	actionCtx := protocol.MustGetActionCtx(ctx)
	blkCtx := protocol.MustGetBlockCtx(ctx)

	topics := []hash.Hash256{hash.Hash256b([]byte(handlerName))}
	if candidateAddr != nil {
		topics = append(topics, hash.Hash256b(candidateAddr.Bytes()))
	}
	topics = append(topics, hash.Hash256b(voterAddr.Bytes()))

	h := hash.Hash160b([]byte(protocolID))
	addr, _ := address.FromBytes(h[:])
	return &action.Log{
		Address:     addr.String(),
		Topics:      topics,
		Data:        data,
		BlockHeight: blkCtx.BlockHeight,
		ActionHash:  actionCtx.ActionHash,
	}
}
