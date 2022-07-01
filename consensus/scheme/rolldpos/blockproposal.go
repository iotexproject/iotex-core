// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"context"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/endorsement"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"google.golang.org/protobuf/proto"
)

type blockProposal struct {
	block       *block.Block
	proofOfLock []*endorsement.Endorsement
}

func newBlockProposal(blk *block.Block, pol []*endorsement.Endorsement) *blockProposal {
	return &blockProposal{
		block:       blk,
		proofOfLock: pol,
	}
}

func (bp *blockProposal) Height() uint64 {
	return bp.block.Height()
}

func (bp *blockProposal) Proto() (*iotextypes.BlockProposal, error) {
	bPb := bp.block.ConvertToBlockPb()
	endorsements := []*iotextypes.Endorsement{}
	for _, en := range bp.proofOfLock {
		ePb, err := en.Proto()
		if err != nil {
			return nil, err
		}
		endorsements = append(endorsements, ePb)
	}
	return &iotextypes.BlockProposal{
		Block:        bPb,
		Endorsements: endorsements,
	}, nil
}

func (bp *blockProposal) Hash() ([]byte, error) {
	msg, err := bp.Proto()
	if err != nil {
		return nil, err
	}
	h := hash.Hash256b(byteutil.Must(proto.Marshal(msg)))

	return h[:], nil
}

func (bp *blockProposal) ProposerAddress() string {
	return bp.block.ProducerAddress()
}

func (bp *blockProposal) LoadProto(ctx context.Context, msg *iotextypes.BlockProposal) error {
	evmNetworkID := mustGetEvmNetworkIDFromCtx(ctx)
	blk, err := block.NewDeserializer(evmNetworkID).FromBlockProto(msg.Block)
	if err != nil {
		return err
	}
	bp.block = blk
	bp.proofOfLock = []*endorsement.Endorsement{}
	for _, ePb := range msg.Endorsements {
		en := &endorsement.Endorsement{}
		if err := en.LoadProto(ePb); err != nil {
			return err
		}
		bp.proofOfLock = append(bp.proofOfLock, en)
	}
	return nil
}
