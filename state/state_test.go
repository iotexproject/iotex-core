// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided   'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to   the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed   by Apache
// License 2.0 that can be found in the LICENSE file.

package state

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/test/identityset"
)

func TestSerializeAndDeserialize(t *testing.T) {
	r := require.New(t)

	s1 := struct{}{}
	r.Panics(func() { Serialize(&s1) }, "data holder doesn't implement Serializer interface!")
	r.Panics(func() { Deserialize(s1, []byte{0xa, 0x2e}) }, "data holder doesn't implement Deserializer interface!")

	list1 := CandidateList{
		&Candidate{
			Address: identityset.Address(2).String(),
			Votes:   big.NewInt(2),
		},
	}
	bytes, err := Serialize(&list1)
	r.NoError(err)

	var list2 CandidateList
	err = Deserialize(&list2, bytes)
	r.NoError(err)
	r.Equal(list2.Len(), list1.Len())
	for i, c := range list2 {
		r.True(c.Equal(list1[i]))
	}
}
