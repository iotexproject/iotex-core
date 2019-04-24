// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateActiveProtocols(t *testing.T) {
	b := UpdateActiveProtocolsBuilder{}
	uap1 := b.SetAdditions([]string{"a", "b"}).
		SetRemovals([]string{"c", "d"}).
		SetData([]byte("data")).
		Build()
	proto := uap1.Proto()
	uap2 := UpdateActiveProtocols{}
	require.NoError(t, uap2.LoadProto(proto))
	assert.EqualValues(t, uap1.additions, uap2.additions)
	assert.EqualValues(t, uap1.removals, uap2.removals)
	assert.EqualValues(t, uap1.data, uap2.data)
}
