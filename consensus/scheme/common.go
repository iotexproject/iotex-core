package scheme

import (
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/consensus/endorsementmanager"
	"github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/endorsement"
	"github.com/iotexproject/iotex-core/state"
)

// ErrInsufficientEndorsements represents the error that not enough endorsements
var ErrInsufficientEndorsements = errors.New("Insufficient endorsements")

// CandidatesByHeightFunc defines a function to overwrite candidates
type CandidatesByHeightFunc func(uint64) ([]*state.Candidate, error)

// GetDelegates gets delegates by block height
func GetDelegates(rp *rolldpos.Protocol, candidatesByHeightFunc CandidatesByHeightFunc, height uint64) ([]string, error) {
	return getDelegates(rp, candidatesByHeightFunc, height)
}

// IsDelegate checks whether the block producer is a delegate
func IsDelegate(delegates []string, addr string) bool {
	return isDelegate(delegates, addr)
}

// AddVoteEndorsement adds a vote endorsement
func AddVoteEndorsement(
	eManager *endorsementmanager.EndorsementManager,
	vote *endorsementmanager.ConsensusVote,
	en *endorsement.Endorsement,
) error {
	return addVoteEndorsement(eManager, vote, en)
}

// IsMajority checks whether the endorsers are the majority
func IsMajority(endorsements []*endorsement.Endorsement, numDelegates int) bool {
	return isMajority(endorsements, numDelegates)
}

// EndorsedByMajority checks wether the endorsers are the majority
func EndorsedByMajority(
	eManager *endorsementmanager.EndorsementManager,
	blockHash []byte,
	topics []endorsementmanager.ConsensusVoteTopic,
	numDelegates int,
) bool {
	return endorsedByMajority(eManager, blockHash, topics, numDelegates)
}

// Endorsements returns endorsements
func Endorsements(
	eManager *endorsementmanager.EndorsementManager,
	blkHash []byte,
	topics []endorsementmanager.ConsensusVoteTopic,
) []*endorsement.Endorsement {
	return endorsements(eManager, blkHash, topics)
}

// Block gets block by block hash
func Block(eManager *endorsementmanager.EndorsementManager, blkHash []byte) *block.Block {
	return getBlock(eManager, blkHash)
}

func getDelegates(rp *rolldpos.Protocol, candidatesByHeightFunc CandidatesByHeightFunc, height uint64) ([]string, error) {
	epochStartHeight := rp.GetEpochHeight(rp.GetEpochNum(height))
	numDelegates := rp.NumDelegates()
	candidates, err := candidatesByHeightFunc(epochStartHeight)
	if err != nil {
		return nil, errors.Wrapf(
			err,
			"failed to get candidates on height %d",
			epochStartHeight,
		)
	}
	if len(candidates) < int(numDelegates) {
		return nil, errors.Errorf(
			"# of candidates %d is less than from required number %d",
			len(candidates),
			numDelegates,
		)
	}
	var addrs []string
	for i, candidate := range candidates {
		if uint64(i) >= rp.NumCandidateDelegates() {
			break
		}
		addrs = append(addrs, candidate.Address)
	}
	crypto.SortCandidates(addrs, epochStartHeight, crypto.CryptoSeed)

	return addrs[:numDelegates], nil
}

func isDelegate(delegates []string, addr string) bool {
	for _, d := range delegates {
		if addr == d {
			return true
		}
	}
	return false
}

func addVoteEndorsement(eManager *endorsementmanager.EndorsementManager,
	vote *endorsementmanager.ConsensusVote,
	en *endorsement.Endorsement,
) error {
	if !endorsement.VerifyEndorsement(vote, en) {
		return errors.New("invalid endorsement for the vote")
	}
	blockHash := vote.BlockHash()
	// TODO: (zhi) request for block
	if len(blockHash) != 0 && getBlock(eManager, blockHash) == nil {
		return errors.New("the corresponding block not received")
	}
	if err := eManager.AddVoteEndorsement(vote, en); err != nil {
		return err
	}
	return nil
}

func isMajority(endorsements []*endorsement.Endorsement, numDelegates int) bool {
	return 3*len(endorsements) > 2*numDelegates
}

func endorsedByMajority(
	eManager *endorsementmanager.EndorsementManager,
	blockHash []byte,
	topics []endorsementmanager.ConsensusVoteTopic,
	numDelegates int,
) bool {
	return isMajority(endorsements(eManager, blockHash, topics), numDelegates)
}

func endorsements(
	eManager *endorsementmanager.EndorsementManager,
	blkHash []byte,
	topics []endorsementmanager.ConsensusVoteTopic,
) []*endorsement.Endorsement {
	c := eManager.CollectionByBlockHash(blkHash)
	if c == nil {
		return []*endorsement.Endorsement{}
	}
	return c.Endorsements(topics)
}

func getBlock(eManager *endorsementmanager.EndorsementManager, blkHash []byte) *block.Block {
	c := eManager.CollectionByBlockHash(blkHash)
	if c == nil {
		return nil
	}
	return c.Block()
}
