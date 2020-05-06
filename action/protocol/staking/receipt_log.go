// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
)

type receiptLog struct {
	addr                  string
	topics                action.Topics
	data                  []byte
	postFairbankMigration bool
}

func newReceiptLog(addr, topic string, postFairbankMigration bool) *receiptLog {
	r := receiptLog{
		addr:                  addr,
		postFairbankMigration: postFairbankMigration,
	}

	if postFairbankMigration {
		r.topics = action.Topics{hash.BytesToHash256([]byte(topic))}
	} else {
		r.topics = action.Topics{hash.Hash256b([]byte(topic))}
	}
	return &r
}

func (r *receiptLog) AddTopics(topics ...[]byte) {
	if r.postFairbankMigration {
		for i := range topics {
			r.topics = append(r.topics, hash.BytesToHash256(topics[i]))
		}
	}
}

func (r *receiptLog) AddAddress(addr address.Address) {
	if !r.postFairbankMigration && addr != nil {
		r.topics = append(r.topics, hash.Hash256b(addr.Bytes()))
	}
}

func (r *receiptLog) SetData(data []byte) {
	r.data = data
}

func (r *receiptLog) Build(ctx context.Context, err error) *action.Log {
	blkCtx := protocol.MustGetBlockCtx(ctx)
	actionCtx := protocol.MustGetActionCtx(ctx)

	log := action.Log{
		Address:     r.addr,
		Topics:      r.topics,
		BlockHeight: blkCtx.BlockHeight,
		ActionHash:  actionCtx.ActionHash,
	}

	if r.postFairbankMigration {
		return &log
	}

	if err == nil {
		log.Data = r.data
		return &log
	}
	return nil
}
