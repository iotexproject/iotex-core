// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package vote

import (
	"sort"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"
)

//Blacklist defines a map where key is candidate's name and value is the counter which counts the unproductivity during kick-out epoch.
type Blacklist struct {
	BlacklistInfos map[string]uint32
	IntensityRate  float64
}

// Serialize serializes map of blacklist to bytes
func (bl *Blacklist) Serialize() ([]byte, error) {
	return proto.Marshal(bl.Proto())
}

// Proto converts the blacklist to a protobuf message
func (bl *Blacklist) Proto() *iotextypes.KickoutCandidateList {
	kickoutListPb := make([]*iotextypes.KickoutInfo, 0, len(bl.BlacklistInfos))
	names := make([]string, 0, len(bl.BlacklistInfos))
	for name := range bl.BlacklistInfos {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		kickoutListPb = append(kickoutListPb, &iotextypes.KickoutInfo{
			Address: name,
			Count:   bl.BlacklistInfos[name],
		})
	}
	return &iotextypes.KickoutCandidateList{
		Blacklists:    kickoutListPb,
		IntensityRate: uint32(bl.IntensityRate * 100),
	}
}

// Deserialize deserializes bytes to delegate blacklist
func (bl *Blacklist) Deserialize(buf []byte) error {
	blackList := &iotextypes.KickoutCandidateList{}
	if err := proto.Unmarshal(buf, blackList); err != nil {
		return errors.Wrap(err, "failed to unmarshal blacklist")
	}
	return bl.LoadProto(blackList)
}

// LoadProto loads blacklist from proto
func (bl *Blacklist) LoadProto(blackListpb *iotextypes.KickoutCandidateList) error {
	blackListMap := make(map[string]uint32, 0)
	candidates := blackListpb.Blacklists
	for _, cand := range candidates {
		blackListMap[cand.Address] = cand.Count
	}
	bl.BlacklistInfos = blackListMap
	bl.IntensityRate = float64(blackListpb.IntensityRate) / float64(100)

	return nil
}
