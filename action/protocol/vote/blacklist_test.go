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
	blacklist1 := Blacklist{}
	blacklist1["addr1"] = 1
	blacklist1["addr2"] = 2
	blacklist1["addr3"] = 1

	sbytes, err := blacklist1.Serialize()
	r.NoError(err)

	blacklist2 := Blacklist{}
	err = blacklist2.Deserialize(sbytes)
	r.NoError(err)
	r.Equal(3, len(blacklist2))
	for addr, count := range blacklist2 {
		r.True(count == blacklist1[addr])
	}
}
