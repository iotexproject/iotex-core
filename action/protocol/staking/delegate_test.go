package staking

import (
	"math/big"
	"testing"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/test/identityset"
)

func TestDelegate(t *testing.T) {
	require := require.New(t)

	d, err := NewDelegate("", "", "", "testname", "a2100000000", "")
	require.Error(err)
	d, err = NewDelegate("", "", "", "testname1234", "a2100000000", "")
	require.Equal(ErrInvalidAmount, errors.Cause(err))
	d, err = NewDelegate("", "", "", "testname1234", "-2100000000", "")
	require.Equal(ErrInvalidAmount, errors.Cause(err))
	d, err = NewDelegate("", "", "", "testname1234", "2100000000", "a2100000000")
	require.Equal(ErrInvalidAmount, errors.Cause(err))
	d, err = NewDelegate("", "", "", "testname1234", "2100000000", "-2100000000")
	require.Equal(ErrInvalidAmount, errors.Cause(err))
	d, err = NewDelegate("d390*jk jh{}", "", "", "testname1234", "2100000000", "2100000000")
	require.Error(err)
	d, err = NewDelegate("io14s0vgnj0pjnazu4hsqlksdk7slah9vcfscn9ks", "d390*jk jh{}", "", "testname1234", "2100000000", "2100000000")
	require.Error(err)
	d, err = NewDelegate("io14s0vgnj0pjnazu4hsqlksdk7slah9vcfscn9ks", "io14s0vgnj0pjnazu4hsqlksdk7slah9vcfscn9ks", "d390*jk jh{}", "testname1234", "2100000000", "2100000000")
	require.Error(err)
	d, err = NewDelegate("io14s0vgnj0pjnazu4hsqlksdk7slah9vcfscn9ks", "io14s0vgnj0pjnazu4hsqlksdk7slah9vcfscn9ks", "io14s0vgnj0pjnazu4hsqlksdk7slah9vcfscn9ks", "testname1234", "2100000000", "2100000000")
	require.NoError(err)

	b, err := d.Serialize()
	require.NoError(err)
	d1 := &Delegate{}
	require.NoError(d1.Deserialize(b))
	require.Equal(d, d1)
}

func TestCandMap(t *testing.T) {
	r := require.New(t)

	tests := []struct {
		d     *Delegate
		index int
	}{
		{
			&Delegate{
				Owner:         hash.Hash160b([]byte("test1")),
				Address:       identityset.Address(2).String(),
				RewardAddress: "io1066kus4vlyvk0ljql39fzwqw0k22h7j8wmef3n",
				CanName:       ToCandName([]byte("test1")),
				Votes:         big.NewInt(2),
				SelfStake:     big.NewInt(1200000),
			},
			2,
		},
		{
			&Delegate{
				Owner:         hash.Hash160b([]byte("test2")),
				Address:       identityset.Address(3).String(),
				RewardAddress: "io1757z4d53408usrx2nf2vr5jh0mc5f5qm8nkre2",
				CanName:       ToCandName([]byte("test2")),
				Votes:         big.NewInt(3),
				SelfStake:     big.NewInt(1200000),
			},
			0,
		},
		{
			&Delegate{
				Owner:         hash.Hash160b([]byte("test4")),
				Address:       identityset.Address(3).String(),
				RewardAddress: "io1757z4d53408usrx2nf2vr5jh0mc5f5qm8nkre2",
				CanName:       ToCandName([]byte("test4")),
				Votes:         big.NewInt(3),
				SelfStake:     big.NewInt(1200000),
			},
			1,
		},
		{
			&Delegate{
				Owner:         hash.Hash160b([]byte("test3")),
				Address:       identityset.Address(3).String(),
				RewardAddress: "io1mflp9m6hcgm2qcghchsdqj3z3eccrnekx9p0ms",
				CanName:       ToCandName([]byte("test3")),
				Votes:         big.NewInt(1),
				SelfStake:     big.NewInt(1200000),
			},
			3,
		},
	}

	m := DelegateMap{}
	for i, v := range tests {
		m[v.d.Owner] = tests[i].d
		r.True(m.Contains(v.d.Owner))
	}
	r.Equal(len(tests), len(m))

	d, err := m.Serialize()
	r.NoError(err)
	r.NoError(m.Deserialize(d))
	r.Equal(len(tests), len(m))
	for _, v := range tests {
		r.True(m.Contains(v.d.Owner))
		r.Equal(v.d, m[v.d.Owner])
	}

	// verify the serialization is sorted
	c := DelegateList{tests[0].d}
	r.NoError(c.Deserialize(d))
	r.Equal(len(tests), len(c))
	for _, v := range tests {
		r.Equal(v.d, c[v.index])
	}
}
