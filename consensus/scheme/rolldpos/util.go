package rolldpos

import (
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/endorsement"
)

func addVoteEndorsement(eManager *endorsementManager,
	vote *ConsensusVote,
	en *endorsement.Endorsement,
	numDelegates int,
) error {
	if !endorsement.VerifyEndorsement(vote, en) {
		return errors.New("invalid endorsement for the vote")
	}
	blockHash := vote.BlockHash()
	// TODO: (zhi) request for block
	if len(blockHash) != 0 && fetchBlock(eManager, blockHash) == nil {
		return errors.New("the corresponding block not received")
	}
	if err := eManager.AddVoteEndorsement(vote, en, numDelegates); err != nil {
		return err
	}
	return nil
}

func endorsements(
	eManager *endorsementManager,
	blkHash []byte,
	topics []ConsensusVoteTopic,
) []*endorsement.Endorsement {
	c := eManager.CollectionByBlockHash(blkHash)
	if c == nil {
		return []*endorsement.Endorsement{}
	}
	return c.Endorsements(topics)
}

func isMajority(endorsements []*endorsement.Endorsement, numDelegates int) bool {
	return 3*len(endorsements) > 2*numDelegates
}

func endorsedByMajority(
	eManager *endorsementManager,
	blockHash []byte,
	topics []ConsensusVoteTopic,
	numDelegates int,
) bool {
	return isMajority(endorsements(eManager, blockHash, topics), numDelegates)
}

func fetchBlock(eManager *endorsementManager, blkHash []byte) *block.Block {
	c := eManager.CollectionByBlockHash(blkHash)
	if c == nil {
		return nil
	}
	return c.Block()
}
