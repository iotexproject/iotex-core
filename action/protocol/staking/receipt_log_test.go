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

	h := hash.Hash160b([]byte(_protocolID))
	addr, _ := address.FromBytes(h[:])
	cand := identityset.Address(5)
	voter := identityset.Address(11)
	index := byteutil.Uint64ToBytesBigEndian(1)
	b1 := cand.Bytes()
	b2 := voter.Bytes()

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
			addr.String(),
			HandleCreateStake,
			[][]byte{index, b1},
			cand,
			voter,
			index,
		},
		{
			addr.String(),
			HandleUnstake,
			[][]byte{index, b1},
			nil,
			voter,
			nil,
		},
		{
			addr.String(),
			HandleWithdrawStake,
			[][]byte{index, b1},
			nil,
			voter,
			nil,
		},
		{
			addr.String(),
			HandleChangeCandidate,
			[][]byte{index, b1, b2},
			cand,
			voter,
			nil,
		},
		{
			addr.String(),
			HandleTransferStake,
			[][]byte{index, b2, b1},
			nil,
			voter,
			nil,
		},
		{
			addr.String(),
			HandleDepositToStake,
			[][]byte{index, b2, b1},
			nil,
			voter,
			nil,
		},
		{
			addr.String(),
			HandleRestake,
			[][]byte{index, b1},
			nil,
			voter,
			nil,
		},
		{
			addr.String(),
			HandleCandidateRegister,
			[][]byte{index, b1},
			cand,
			voter,
			index,
		},
		{
			addr.String(),
			HandleCandidateUpdate,
			[][]byte{index, b1},
			nil,
			voter,
			nil,
		},
	}

	postFb := &action.Log{
		Address:     addr.String(),
		BlockHeight: 6,
		ActionHash:  hash.Hash256b([]byte("test-action")),
	}

	for _, v := range logTests {
		log := newReceiptLog(v.addr, v.name, false)
		log.AddTopics(v.topics...)
		log.AddAddress(v.cand)
		log.AddAddress(v.voter)
		log.SetData(v.data)
		r.Nil(log.Build(ctx, ErrInvalidAmount))
		r.Equal(createLog(ctx, v.name, v.cand, v.voter, v.data), log.Build(ctx, nil))

		log = newReceiptLog(v.addr, v.name, true)
		log.AddTopics(v.topics...)
		log.AddAddress(v.cand)
		log.AddAddress(v.voter)
		log.SetData(v.data)
		postFb.Topics = action.Topics{hash.BytesToHash256([]byte(v.name))}
		for i := range v.topics {
			postFb.Topics = append(postFb.Topics, hash.BytesToHash256(v.topics[i]))
		}
		r.Equal(postFb, log.Build(ctx, ErrInvalidAmount))
		r.Equal(postFb, log.Build(ctx, nil))
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

	h := hash.Hash160b([]byte(_protocolID))
	addr, _ := address.FromBytes(h[:])
	return &action.Log{
		Address:     addr.String(),
		Topics:      topics,
		Data:        data,
		BlockHeight: blkCtx.BlockHeight,
		ActionHash:  actionCtx.ActionHash,
	}
}
