// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package vote

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProbationListSerializeAndDeserialize(t *testing.T) {
	r := require.New(t)
	probationList1 := &ProbationList{
		ProbationInfo: make(map[string]uint32, 0),
	}

	probationList1.ProbationInfo["addr1"] = 1
	probationList1.ProbationInfo["addr2"] = 2
	probationList1.ProbationInfo["addr3"] = 1
	probationList1.IntensityRate = 50
	sbytes, err := probationList1.Serialize()
	r.NoError(err)

	probationList2 := &ProbationList{}
	err = probationList2.Deserialize(sbytes)
	r.NoError(err)
	r.Equal(3, len(probationList2.ProbationInfo))
	for addr, count := range probationList2.ProbationInfo {
		r.True(count == probationList1.ProbationInfo[addr])
	}

	r.True(probationList2.IntensityRate == probationList1.IntensityRate)
	r.True(probationList2.IntensityRate == 50)

	probationList3 := &ProbationList{}
	sbytes, err = probationList3.Serialize()
	r.NoError(err)
	probationList4 := &ProbationList{}
	err = probationList4.Deserialize(sbytes)
	r.NoError(err)

	r.True(len(probationList4.ProbationInfo) == 0)
}
