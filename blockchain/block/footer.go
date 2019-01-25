// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package block

import (
	"github.com/iotexproject/iotex-core/endorsement"
	"github.com/iotexproject/iotex-core/proto"
)

// Footer defines a set of proof of this block
type Footer struct {
	// endorsements contain COMMIT endorsements from more than 2/3 delegates
	endorsements    *endorsement.Set
	commitTimestamp int64
}

// ConvertToBlockFooterPb converts BlockFooterPb
func (f *Footer) ConvertToBlockFooterPb() *iproto.BlockFooterPb {
	pb := iproto.BlockFooterPb{}
	pb.CommitTimestamp = f.commitTimestamp
	if f.endorsements != nil {
		pb.Endorsements = f.endorsements.ToProto()
	}

	return &pb
}

// ConvertFromBlockFooterPb converts BlockFooterPb to BlockFooter
func (f *Footer) ConvertFromBlockFooterPb(pb *iproto.BlockFooterPb) error {
	f.commitTimestamp = pb.GetCommitTimestamp()
	pbEndorsements := pb.GetEndorsements()
	if pbEndorsements == nil {
		return nil
	}
	f.endorsements = &endorsement.Set{}

	return f.endorsements.FromProto(pbEndorsements)
}

// CommitTime returns the timestamp the block was committed
func (f *Footer) CommitTime() int64 {
	return f.commitTimestamp
}

// NumOfDelegateEndorsements returns the number of commit endorsements froms delegates
func (f *Footer) NumOfDelegateEndorsements(delegates []string) int {
	if f.endorsements == nil {
		return 0
	}
	return f.endorsements.NumOfValidEndorsements(
		map[endorsement.ConsensusVoteTopic]bool{endorsement.COMMIT: true},
		delegates,
	)
}
