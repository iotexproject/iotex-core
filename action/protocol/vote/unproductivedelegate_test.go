// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package vote

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUnproductiveDelegate(t *testing.T) {
	r := require.New(t)
	upd, err := NewUnproductiveDelegate(2, 10)
	r.NoError(err)

	str1 := []string{"a", "b", "c", "d", "e"}
	str2 := []string{"d"}
	str3 := []string{"f", "g"}
	str4 := []string{"a", "f", "g"}

	r.NoError(upd.AddRecentUPD(str1))
	r.NoError(upd.AddRecentUPD(str2))
	r.NoError(upd.AddRecentUPD(str3))
	oldestData := upd.ReadOldestUPD()
	r.Equal(len(str2), len(oldestData))
	for i, data := range oldestData {
		r.Equal(str2[i], data)
	}

	r.NoError(upd.AddRecentUPD(str4))
	oldestData = upd.ReadOldestUPD()
	r.Equal(len(str3), len(oldestData))
	for i, data := range oldestData {
		r.Equal(str3[i], data)
	}

	sbytes, err := upd.Serialize()
	r.NoError(err)

	upd2, err := NewUnproductiveDelegate(3, 10)
	r.NoError(err)
	err = upd2.Deserialize(sbytes)
	r.NoError(err)

	r.True(upd.Equal(upd2))
}
