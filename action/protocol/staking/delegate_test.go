package staking

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/test/identityset"
)

func TestCandMap(t *testing.T) {
	r := require.New(t)

	tests := []struct {
		d     *Delegate
		index int
	}{
		{
			&Delegate{
				Owner:         "io1066kus4vlyvk0ljql39fzwqw0k22h7j8wmef3n",
				Address:       identityset.Address(2).String(),
				Votes:         big.NewInt(2),
				RewardAddress: "io1066kus4vlyvk0ljql39fzwqw0k22h7j8wmef3n",
				CanName:       ToCandName([]byte("test1")),
			},
			2,
		},
		{
			&Delegate{
				Owner:         "io1757z4d53408usrx2nf2vr5jh0mc5f5qm8nkre2",
				Address:       identityset.Address(3).String(),
				Votes:         big.NewInt(3),
				RewardAddress: "io1757z4d53408usrx2nf2vr5jh0mc5f5qm8nkre2",
				CanName:       ToCandName([]byte("test2")),
			},
			1,
		},
		{
			&Delegate{
				Owner:         "io1d4c5lp4ea4754wy439g2t99ue7wryu5r2lslh2",
				Address:       identityset.Address(3).String(),
				Votes:         big.NewInt(3),
				RewardAddress: "io1757z4d53408usrx2nf2vr5jh0mc5f5qm8nkre2",
				CanName:       ToCandName([]byte("test4")),
			},
			0,
		},
		{
			&Delegate{
				Owner:         "io1mflp9m6hcgm2qcghchsdqj3z3eccrnekx9p0ms",
				Address:       identityset.Address(3).String(),
				Votes:         big.NewInt(1),
				RewardAddress: "io1mflp9m6hcgm2qcghchsdqj3z3eccrnekx9p0ms",
				CanName:       ToCandName([]byte("test3")),
			},
			3,
		},
	}

	m := DelegateMap{}
	for i, v := range tests {
		m[v.d.CanName] = tests[i].d
		r.True(m.Contains(v.d.CanName))
	}
	r.Equal(len(tests), len(m))

	d, err := m.Serialize()
	r.NoError(err)
	r.NoError(m.Deserialize(d))
	r.Equal(len(tests), len(m))
	for _, v := range tests {
		r.True(m.Contains(v.d.CanName))
		r.Equal(v.d, m[v.d.CanName])
	}

	// verify the serialization is sorted
	c := DelegateList{tests[0].d}
	r.NoError(c.Deserialize(d))
	r.Equal(len(tests), len(c))
	for _, v := range tests {
		r.Equal(v.d, c[v.index])
	}
}
