// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/consensus/fsm"
	"github.com/iotexproject/iotex-core/pkg/hash"
	pb "github.com/iotexproject/iotex-core/proto"
)

func eventFromProto(m proto.Message) (*fsm.Event, error) {
	event := &fsm.Event{}
	vc, ok := (m).(*pb.ViewChangeMsg)
	if !ok {
		return event, errors.Wrapf(ErrInvalidViewChangeMsg, "message content is %+v", m.String())
	}

	event.State = fsm.State(vc.GetVctype().String())

	event.SenderAddr = vc.GetSenderAddr()

	blkPb := vc.GetBlock()
	if blkPb != nil {
		blk := &blockchain.Block{}
		blk.ConvertFromBlockPb(blkPb)
		event.Block = blk
	}

	if blkHashPb := vc.GetBlockHash(); blkHashPb != nil {
		event.BlockHash = &hash.Hash32B{}
		copy(event.BlockHash[:], blkHashPb)
	}
	return event, nil
}

func protoFromEvent(event *fsm.Event) proto.Message {
	msg := &pb.ViewChangeMsg{}
	switch event.State {
	case stateAcceptVote:
		msg.Vctype = pb.ViewChangeMsg_VOTE
	case stateAcceptPrevote:
		msg.Vctype = pb.ViewChangeMsg_PREVOTE
	case stateAcceptPropose:
		msg.Vctype = pb.ViewChangeMsg_PROPOSE
	}
	if event.Block != nil {
		msg.Block = event.Block.ConvertToBlockPb()
	}
	if event.BlockHash != nil {
		msg.BlockHash = event.BlockHash[:]
	}
	msg.SenderAddr = event.SenderAddr
	return msg
}
