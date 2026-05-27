// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/endorsement"
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"google.golang.org/protobuf/proto"
)

type blockProposal struct {
	block       *block.Block
	proofOfLock *ProofOfLock
}

func newBlockProposal(blk *block.Block, pol *ProofOfLock) *blockProposal {
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
	out := &iotextypes.BlockProposal{Block: bPb}
	for _, en := range bp.proofOfLock.Endorsements() {
		out.Endorsements = append(out.Endorsements, en.Proto())
	}
	for _, en := range bp.proofOfLock.BLSEndorsements() {
		out.BlsEndorsements = append(out.BlsEndorsements, en.Proto())
	}
	return out, nil
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

func (bp *blockProposal) LoadProto(msg *iotextypes.BlockProposal, deserializer *block.Deserializer) error {
	blk, err := deserializer.FromBlockProto(msg.Block)
	if err != nil {
		return err
	}
	bp.block = blk
	if len(msg.BlsEndorsements) > 0 {
		bls := make([]*endorsement.BLSEndorsement, 0, len(msg.BlsEndorsements))
		for _, ePb := range msg.BlsEndorsements {
			en := &endorsement.BLSEndorsement{}
			if err := en.LoadProto(ePb); err != nil {
				return err
			}
			bls = append(bls, en)
		}
		bp.proofOfLock = NewBLSProofOfLock(bls)
		return nil
	}
	ens := make([]*endorsement.Endorsement, 0, len(msg.Endorsements))
	for _, ePb := range msg.Endorsements {
		en := &endorsement.Endorsement{}
		if err := en.LoadProto(ePb); err != nil {
			return err
		}
		ens = append(ens, en)
	}
	bp.proofOfLock = NewProofOfLock(ens)
	return nil
}
