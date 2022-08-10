// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package vote

import (
	"sort"

	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"
)

// ProbationList defines a map where key is candidate's name and value is the counter which counts the unproductivity during probation epoch.
type ProbationList struct {
	ProbationInfo map[string]uint32
	IntensityRate uint32
}

// NewProbationList returns a new probation list
func NewProbationList(intensity uint32) *ProbationList {
	return &ProbationList{
		ProbationInfo: make(map[string]uint32),
		IntensityRate: intensity,
	}
}

// Serialize serializes map of ProbationList to bytes
func (pl *ProbationList) Serialize() ([]byte, error) {
	return proto.Marshal(pl.Proto())
}

// Proto converts the ProbationList to a protobuf message
func (pl *ProbationList) Proto() *iotextypes.ProbationCandidateList {
	probationListPb := make([]*iotextypes.ProbationCandidateList_Info, 0, len(pl.ProbationInfo))
	names := make([]string, 0, len(pl.ProbationInfo))
	for name := range pl.ProbationInfo {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		probationListPb = append(probationListPb, &iotextypes.ProbationCandidateList_Info{
			Address: name,
			Count:   pl.ProbationInfo[name],
		})
	}
	return &iotextypes.ProbationCandidateList{
		ProbationList: probationListPb,
		IntensityRate: pl.IntensityRate,
	}
}

// Deserialize deserializes bytes to delegate ProbationList
func (pl *ProbationList) Deserialize(buf []byte) error {
	ProbationList := &iotextypes.ProbationCandidateList{}
	if err := proto.Unmarshal(buf, ProbationList); err != nil {
		return errors.Wrap(err, "failed to unmarshal probationList")
	}
	return pl.LoadProto(ProbationList)
}

// LoadProto loads ProbationList from proto
func (pl *ProbationList) LoadProto(probationListpb *iotextypes.ProbationCandidateList) error {
	probationlistMap := make(map[string]uint32, 0)
	candidates := probationListpb.ProbationList
	for _, cand := range candidates {
		probationlistMap[cand.Address] = cand.Count
	}
	pl.ProbationInfo = probationlistMap
	pl.IntensityRate = probationListpb.IntensityRate

	return nil
}
