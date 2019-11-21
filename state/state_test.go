// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided   'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to   the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed   by Apache
// License 2.0 that can be found in the LICENSE file.

package state

import (
    "testing"
    "math/big"

    "github.com/stretchr/testify/require"

    "github.com/iotexproject/iotex-core/test/identityset"
)

func TestSerialize(t *testing.T) {
    require := require.New(t)

    s1 := struct{}{}
    require.Panics(func() { Serialize(&s1) }, "data holder doesn't implement Serializer interface!")

    list := CandidateList{
        &Candidate{
            Address: identityset.Address(2).String(),
            Votes:   big.NewInt(2),
        },
    }
    data := []byte{0xa, 0x2e, 0xa, 0x29, 0x69, 0x6f, 0x31, 0x68, 0x68, 0x39, 0x37, 0x66, 0x32, 0x37, 0x33, 0x6e, 0x68, 0x78, 0x63, 0x71, 0x38, 0x61, 0x6a, 0x7a, 0x63, 0x70, 0x75, 0x6a, 0x74, 0x74, 0x37, 0x70, 0x39, 0x70, 0x71, 0x79, 0x6e, 0x64, 0x66, 0x6d, 0x61, 0x76, 0x6e, 0x39, 0x72, 0x12, 0x1, 0x2}
    res, err := Serialize(&list)
    require.Equal(nil, err)
    require.Equal(res, data)
}

func TestDeserialize(t *testing.T) {
    require := require.New(t)
    s1 := struct{}{}
    require.Panics(func() { Deserialize(s1, []byte{0xa, 0x2e}) }, "data holder doesn't implement Deserializer interface!")
    data := []byte{0xa, 0x2e, 0xa, 0x29, 0x69, 0x6f, 0x31, 0x68, 0x68, 0x39, 0x37, 0x66, 0x32, 0x37, 0x33, 0x6e, 0x68, 0x78, 0x63, 0x71, 0x38, 0x61, 0x6a, 0x7a, 0x63, 0x70, 0x75, 0x6a, 0x74, 0x74, 0x37, 0x70, 0x39, 0x70, 0x71, 0x79, 0x6e, 0x64, 0x66, 0x6d, 0x61, 0x76, 0x6e, 0x39, 0x72, 0x12, 0x1, 0x2}
    list1 := CandidateList{
        &Candidate{
            Address: identityset.Address(2).String(),
            Votes:   big.NewInt(2),
        },
    }
    var list2 CandidateList
    err := Deserialize(&list2, data)
    require.Equal(nil, err)
    for i, cand := range list2 {
        require.Equal(cand, list1[i])
    }
}
