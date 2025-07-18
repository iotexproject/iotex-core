// Copyright (c) 2020 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package vote

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUnproductiveDelegate(t *testing.T) {
	r := require.New(t)
	upd, err := NewUnproductiveDelegate(2, 10)
	r.NoError(err)

	str1 := map[string]uint64{"a": 1, "b": 2, "c": 3, "d": 4, "e": 5}
	str2 := map[string]uint64{"d": 6}
	str3 := map[string]uint64{"f": 7, "g": 8}
	str4 := map[string]uint64{"a": 1, "f": 7, "g": 8}

	r.NoError(upd.AddRecentUPD(str1))
	r.NoError(upd.AddRecentUPD(str2))
	r.NoError(upd.AddRecentUPD(str3))
	oldestData := upd.ReadOldestUPD()
	r.Equal(len(str2), len(oldestData))
	for _, data := range oldestData {
		r.Contains(str2, data)
	}

	r.NoError(upd.AddRecentUPD(str4))
	oldestData = upd.ReadOldestUPD()
	r.Equal(len(str3), len(oldestData))
	for _, data := range oldestData {
		r.Contains(str3, data)
	}

	sbytes, err := upd.Serialize()
	r.NoError(err)

	upd2, err := NewUnproductiveDelegate(3, 10)
	r.NoError(err)
	err = upd2.Deserialize(sbytes)
	r.NoError(err)

	r.True(upd.Equal(upd2))
}
