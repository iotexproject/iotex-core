// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package scheme

import (
	"context"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/consensus/endorsementmanager"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/endorsement"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/state"
)

// ChainManager defines the blockchain interface
type ChainManager interface {
	// CandidatesByHeight returns the candidate list by a given height
	CandidatesByHeight(height uint64) ([]*state.Candidate, error)
}

// Noop is the consensus scheme that does NOT create blocks
type Noop struct {
	chain                  ChainManager
	candidatesByHeightFunc CandidatesByHeightFunc
	eManagerDB             db.KVStore
	eManager               *endorsementmanager.EndorsementManager
	rp                     *rolldpos.Protocol
}

// NewNoop creates a Noop struct
func NewNoop(
	chain ChainManager,
	candidatesByHeightFunc CandidatesByHeightFunc,
	consensusDBConfig config.DB,
	rp *rolldpos.Protocol,
) Scheme {
	var eManagerDB db.KVStore
	if len(consensusDBConfig.DbPath) > 0 {
		eManagerDB = db.NewBoltDB(consensusDBConfig)
	}
	if candidatesByHeightFunc == nil {
		candidatesByHeightFunc = chain.CandidatesByHeight
	}
	return &Noop{
		chain:                  chain,
		candidatesByHeightFunc: candidatesByHeightFunc,
		eManagerDB:             eManagerDB,
		rp:                     rp,
	}
}

// Start does nothing here
func (n *Noop) Start(c context.Context) (err error) {
	if n.eManagerDB != nil {
		if err := n.eManagerDB.Start(c); err != nil {
			return errors.Wrap(err, "Error when starting the collectionDB")
		}
	}
	n.eManager, err = endorsementmanager.NewEndorsementManager(n.eManagerDB)
	return
}

// Stop does nothing here
func (n *Noop) Stop(c context.Context) error {
	if n.eManagerDB != nil {
		return n.eManagerDB.Stop(c)
	}
	return nil
}

// HandleConsensusMsg handles incoming consensus message
func (n *Noop) HandleConsensusMsg(*iotextypes.ConsensusMessage) error {
	log.Logger("consensus").Warn("Noop scheme does not handle incoming consensus message.")
	return nil
}

// Calibrate triggers an event to calibrate consensus context
func (n *Noop) Calibrate(uint64) {}

// ValidateBlockFooter validates the block footer
func (n *Noop) ValidateBlockFooter(blk *block.Block) error {
	delegates, err := getDelegates(n.rp, n.candidatesByHeightFunc, blk.Height())
	if err != nil {
		return err
	}
	if !isDelegate(delegates, blk.ProducerAddress()) {
		return errors.Errorf(
			"block proposer %s is not a valid delegate",
			blk.ProducerAddress(),
		)
	}
	if err := n.eManager.RegisterBlock(blk); err != nil {
		return err
	}
	blkHash := blk.HashBlock()
	for _, en := range blk.Endorsements() {
		if err := n.addVoteEndorsement(
			endorsementmanager.NewConsensusVote(blkHash[:], endorsementmanager.COMMIT),
			en,
			len(delegates),
		); err != nil {
			return err
		}
	}
	if !endorsedByMajority(n.eManager, blkHash[:], []endorsementmanager.ConsensusVoteTopic{endorsementmanager.COMMIT}, len(delegates)) {
		return ErrInsufficientEndorsements
	}
	return nil
}

// Metrics is not implemented for noop scheme
func (n *Noop) Metrics() (ConsensusMetrics, error) {
	return ConsensusMetrics{}, errors.Wrapf(
		ErrNotImplemented,
		"noop scheme does not supported metrics yet",
	)
}

// Activate is not implemented for noop scheme
func (n *Noop) Activate(_ bool) {
	log.S().Warn("Noop scheme could not support activate")
}

// Active is always true for noop scheme
func (n *Noop) Active() bool { return true }

func (n *Noop) addVoteEndorsement(
	vote *endorsementmanager.ConsensusVote,
	en *endorsement.Endorsement,
	numDelegates int,
) error {
	if err := addVoteEndorsement(n.eManager, vote, en); err != nil {
		return err
	}
	if vote.Topic() == endorsementmanager.LOCK {
		return nil
	}
	if !endorsedByMajority(
		n.eManager,
		vote.BlockHash(),
		[]endorsementmanager.ConsensusVoteTopic{endorsementmanager.PROPOSAL, endorsementmanager.COMMIT},
		numDelegates,
	) {
		return nil
	}
	return nil
}
