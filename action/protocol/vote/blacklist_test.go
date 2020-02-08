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

func TestBlackListSerializeAndDeserialize(t *testing.T) {
	r := require.New(t)
	blacklist1 := &Blacklist{
		BlacklistInfos: make(map[string]uint32, 0),
	}

	blacklist1.BlacklistInfos["addr1"] = 1
	blacklist1.BlacklistInfos["addr2"] = 2
	blacklist1.BlacklistInfos["addr3"] = 1
	blacklist1.IntensityRate = 0.5
	sbytes, err := blacklist1.Serialize()
	r.NoError(err)

	blacklist2 := &Blacklist{}
	err = blacklist2.Deserialize(sbytes)
	r.NoError(err)
	r.Equal(3, len(blacklist2.BlacklistInfos))
	for addr, count := range blacklist2.BlacklistInfos {
		r.True(count == blacklist1.BlacklistInfos[addr])
	}

	r.True(blacklist2.IntensityRate == blacklist1.IntensityRate)
	r.True(blacklist2.IntensityRate == 0.5)
}
