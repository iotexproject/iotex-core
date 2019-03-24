// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package block

import (
	"time"

	"github.com/golang/protobuf/ptypes"

	"github.com/iotexproject/iotex-core/endorsement"
	"github.com/iotexproject/iotex-core/protogen/iotextypes"
)

// Footer defines a set of proof of this block
type Footer struct {
	endorsements []*endorsement.Endorsement
	commitTime   time.Time
}

// ConvertToBlockFooterPb converts BlockFooter
func (f *Footer) ConvertToBlockFooterPb() (*iotextypes.BlockFooter, error) {
	pb := iotextypes.BlockFooter{}
	commitTime, err := ptypes.TimestampProto(f.commitTime)
	if err != nil {
		return nil, err
	}
	pb.Timestamp = commitTime
	pb.Endorsements = []*iotextypes.Endorsement{}
	for _, en := range f.endorsements {
		ePb, err := en.Proto()
		if err != nil {
			return nil, err
		}
		pb.Endorsements = append(pb.Endorsements, ePb)
	}
	return &pb, nil
}

// ConvertFromBlockFooterPb converts BlockFooter to BlockFooter
func (f *Footer) ConvertFromBlockFooterPb(pb *iotextypes.BlockFooter) error {
	if pb == nil {
		return nil
	}
	commitTime, err := ptypes.Timestamp(pb.GetTimestamp())
	if err != nil {
		return err
	}
	f.commitTime = commitTime
	pbEndorsements := pb.GetEndorsements()
	if pbEndorsements == nil {
		return nil
	}
	f.endorsements = []*endorsement.Endorsement{}
	for _, ePb := range pbEndorsements {
		e := &endorsement.Endorsement{}
		if err := e.LoadProto(ePb); err != nil {
			return err
		}
		f.endorsements = append(f.endorsements, e)
	}

	return nil
}

// CommitTime returns the timestamp the block was committed
func (f *Footer) CommitTime() time.Time {
	return f.commitTime
}

// Endorsements returns the number of commit endorsements froms delegates
func (f *Footer) Endorsements() []*endorsement.Endorsement {
	return f.endorsements
}
