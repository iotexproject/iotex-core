// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"math/big"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding/distributedlog"
)

// delegateChunkLog gathers one chunk's voter rows for a delegate. Settlement
// pays each voter once across delegates, then expands the contributing shares
// back into delegate-scoped events for off-chain accounting.
type delegateChunkLog struct {
	voters            []address.Address
	recipients        []address.Address
	amounts           []*big.Int
	compoundBucketIDs []uint64
	compounded        []bool
	paid              *big.Int
}

func recordVoterPayout(logs []delegateChunkLog, payout voterCombinedPayout) {
	for _, share := range payout.shares {
		i := share.delegateIndex
		if i < 0 || i >= len(logs) {
			continue
		}
		logs[i].voters = append(logs[i].voters, payout.voter)
		logs[i].recipients = append(logs[i].recipients, payout.recipient)
		logs[i].amounts = append(logs[i].amounts, new(big.Int).Set(share.share))
		logs[i].compoundBucketIDs = append(logs[i].compoundBucketIDs, payout.compoundBucketID)
		logs[i].compounded = append(logs[i].compounded, payout.compounded)
		if logs[i].paid == nil {
			logs[i].paid = new(big.Int)
		}
		logs[i].paid.Add(logs[i].paid, share.share)
	}
}

func voterTransactionLog(payout voterCombinedPayout) *action.TransactionLog {
	if payout.compounded || payout.recipient == nil || isNilOrZero(payout.amount) {
		return nil
	}
	return &action.TransactionLog{
		Type:      iotextypes.TransactionLogType_CLAIM_FROM_REWARDING_FUND,
		Sender:    address.RewardingPoolAddr,
		Recipient: payout.recipient.String(),
		Amount:    new(big.Int).Set(payout.amount),
	}
}

func (p *Protocol) packDelegateChunkLog(
	targetEra uint64,
	work epochDrainDelegateWork,
	rows delegateChunkLog,
	blkHeight uint64,
	actionHash hash.Hash256,
) (*action.Log, error) {
	if len(rows.voters) == 0 {
		return nil, nil
	}
	candID, err := address.FromBytes(work.CandidateIdentifier)
	if err != nil {
		return nil, errors.Wrap(err, "rewarding: decode cursor candidate identifier")
	}
	topics, data, err := distributedlog.Pack(distributedlog.EventArgs{
		Epoch:             targetEra,
		Delegate:          candID,
		VoterAmount:       safeBig(rows.paid),
		Voters:            rows.voters,
		Recipients:        rows.recipients,
		Amounts:           rows.amounts,
		CompoundBucketIDs: rows.compoundBucketIDs,
		Compounded:        rows.compounded,
	})
	if err != nil {
		return nil, errors.Wrap(err, "rewarding: pack DelegateDistributed log")
	}
	return &action.Log{
		Address: p.addr.String(), Topics: topics, Data: data,
		BlockHeight: blkHeight, ActionHash: actionHash,
	}, nil
}
