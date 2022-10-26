package staking

import (
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/test/identityset"
)

// testEqual verifies m contains exactly the list
func testEqual(m *CandidateCenter, l CandidateList) bool {
	for _, v := range l {
		d := m.GetByOwner(v.Owner)
		if d == nil {
			return false
		}
		if !v.Equal(d) {
			return false
		}

		d = m.GetByName(v.Name)
		if d == nil {
			return false
		}
		if !v.Equal(d) {
			return false
		}

		d = m.GetBySelfStakingIndex(v.SelfStakeBucketIdx)
		if d == nil {
			return false
		}
		if !v.Equal(d) {
			return false
		}
	}
	return true
}

// testEqualAllCommit validates candidate center
// with original candidates = old, number of changed cand = change, and number of new cand = increase
func testEqualAllCommit(r *require.Assertions, m *CandidateCenter, old CandidateList, change, increase int,
) (CandidateList, error) {
	// capture all candidates
	size := len(old)
	list := m.All()
	r.Equal(size+increase, len(list))
	r.Equal(size+increase, m.Size())
	all, err := list.toStateCandidateList()
	r.NoError(err)

	// number of changed cand = change
	delta := m.Delta()
	r.Equal(change, len(delta))
	ser, err := delta.Serialize()
	r.NoError(err)

	// test abort the changes
	r.NoError(m.SetDelta(nil))
	r.Equal(size, m.Size())
	// m equal to old list, not equal to current
	r.True(testEqual(m, old))
	r.False(testEqual(m, list))
	r.Nil(m.Delta())

	// test commit
	r.NoError(delta.Deserialize(ser))
	r.NoError(m.SetDelta(delta))
	r.NoError(m.Commit())
	r.NoError(m.Commit()) // commit is idempotent
	r.Equal(size+increase, m.Size())
	// m equal to current list, not equal to old
	r.True(testEqual(m, list))
	r.False(testEqual(m, old))

	// after commit All() is the same
	list = m.All()
	r.Equal(size+increase, len(list))
	r.Equal(size+increase, m.Size())
	all1, err := list.toStateCandidateList()
	r.NoError(err)
	r.Equal(all, all1)
	return list, nil
}

func TestCandCenter(t *testing.T) {
	r := require.New(t)

	m, err := NewCandidateCenter(nil)
	r.NoError(err)
	for i, v := range testCandidates {
		r.NoError(m.Upsert(testCandidates[i].d))
		r.True(m.ContainsName(v.d.Name))
		r.Equal(v.d, m.GetByName(v.d.Name))
	}
	r.Equal(len(testCandidates), m.Size())

	// test export changes and commit
	list := m.Delta()
	r.NotNil(list)
	r.Equal(len(list), m.Size())
	r.True(testEqual(m, list))
	r.NoError(m.SetDelta(list))
	r.NoError(m.Commit())
	r.Equal(len(testCandidates), m.Size())
	r.True(testEqual(m, list))
	old := m.All()
	r.True(testEqual(m, old))

	// test existence
	for _, v := range testCandidates {
		r.True(m.ContainsName(v.d.Name))
		r.True(m.ContainsOwner(v.d.Owner))
		r.True(m.ContainsOperator(v.d.Operator))
		r.True(m.ContainsSelfStakingBucket(v.d.SelfStakeBucketIdx))
		r.Equal(v.d, m.GetByName(v.d.Name))
		r.Equal(v.d, m.GetByOwner(v.d.Owner))
		r.Equal(v.d, m.GetBySelfStakingIndex(v.d.SelfStakeBucketIdx))
	}

	testDeltas := []*Candidate{
		&Candidate{
			Owner:              identityset.Address(1),
			Operator:           identityset.Address(7),
			Reward:             identityset.Address(1),
			Name:               "update1",
			Votes:              big.NewInt(2),
			SelfStakeBucketIdx: 1,
			SelfStake:          unit.ConvertIotxToRau(1200000),
		},
		&Candidate{
			Owner:              identityset.Address(2),
			Operator:           identityset.Address(8),
			Reward:             identityset.Address(1),
			Name:               "test2",
			Votes:              big.NewInt(3),
			SelfStakeBucketIdx: 200,
			SelfStake:          unit.ConvertIotxToRau(1200000),
		},
		&Candidate{
			Owner:              identityset.Address(3),
			Operator:           identityset.Address(6),
			Reward:             identityset.Address(1),
			Name:               "test3",
			Votes:              big.NewInt(3),
			SelfStakeBucketIdx: 3,
			SelfStake:          unit.ConvertIotxToRau(1200000),
		},
		&Candidate{
			Owner:              identityset.Address(4),
			Operator:           identityset.Address(10),
			Reward:             identityset.Address(1),
			Name:               "test4",
			Votes:              big.NewInt(1),
			SelfStakeBucketIdx: 4,
			SelfStake:          unit.ConvertIotxToRau(1100000),
		},

		&Candidate{
			Owner:              identityset.Address(7),
			Operator:           identityset.Address(1),
			Reward:             identityset.Address(1),
			Name:               "new1",
			Votes:              big.NewInt(2),
			SelfStakeBucketIdx: 6,
			SelfStake:          unit.ConvertIotxToRau(1200000),
		},
		&Candidate{
			Owner:              identityset.Address(8),
			Operator:           identityset.Address(2),
			Reward:             identityset.Address(1),
			Name:               "new2",
			Votes:              big.NewInt(3),
			SelfStakeBucketIdx: 7,
			SelfStake:          unit.ConvertIotxToRau(1200000),
		},
		&Candidate{
			Owner:              identityset.Address(9),
			Operator:           identityset.Address(3),
			Reward:             identityset.Address(1),
			Name:               "new3",
			Votes:              big.NewInt(3),
			SelfStakeBucketIdx: 8,
			SelfStake:          unit.ConvertIotxToRau(1200000),
		},
		&Candidate{
			Owner:              identityset.Address(10),
			Operator:           identityset.Address(4),
			Reward:             identityset.Address(1),
			Name:               "new4",
			Votes:              big.NewInt(1),
			SelfStakeBucketIdx: 9,
			SelfStake:          unit.ConvertIotxToRau(1200000),
		},
	}

	for i := range testDeltas {
		r.NoError(m.Upsert(testDeltas[i]))
	}
	list = m.All()
	r.Equal(len(list), m.Size())
	r.True(testEqual(m, list))
	delta := m.Delta()
	// 4 updates + 4 new
	r.Equal(8, len(delta))
	r.Equal(len(testCandidates)+4, m.Size())

	// SetDelta using one's own Delta() does not change a thing
	r.NoError(m.SetDelta(delta))
	r.Equal(len(list), m.Size())
	r.True(testEqual(m, list))
	r.NoError(m.Commit())
	r.Equal(len(list), m.Size())
	r.True(testEqual(m, list))

	// testDeltas[3] only updated self-stake
	td3 := m.GetByName(testDeltas[3].Name)
	r.Equal(td3, testDeltas[3])

	// test update existing and add new
	// each time pick 2 random candidates to update, and add 2 new cand
	for i := 0; i < 10; i++ {
		size := len(list)
		u1 := time.Now().Nanosecond() / 1000 % size
		u2 := (u1 + 1) % size
		d1 := list[u1].Clone()
		d2 := list[u2].Clone()
		// save the name, operator and self-stake
		name1 := d1.Name
		name2 := d2.Name
		op1 := d1.Operator
		op2 := d2.Operator
		self1 := d1.SelfStakeBucketIdx
		self2 := d2.SelfStakeBucketIdx

		// d1 conflict with existing
		conflict := list[(u1+3)%size]
		d1.Name = conflict.Name
		r.Equal(ErrInvalidCanName, m.Upsert(d1))
		d1.Name = name1
		d1.Operator = conflict.Operator
		r.Equal(ErrInvalidOperator, m.Upsert(d1))
		d1.Operator = op1
		d1.SelfStakeBucketIdx = conflict.SelfStakeBucketIdx
		r.Equal(ErrInvalidSelfStkIndex, m.Upsert(d1))
		d1.SelfStakeBucketIdx = self1

		// no change yet, delta must be empty
		r.Nil(m.Delta())

		// upsert(d1)
		d1.Operator = identityset.Address(13 + i*2) // identityset[13]+ don't conflict with existing
		r.NoError(m.Upsert(d1))
		d1.Name = d1.Operator.String()[:12]
		r.NoError(m.Upsert(d1))

		// d2 conflict d1
		d2.Name = d1.Name
		r.Equal(ErrInvalidCanName, m.Upsert(d2))
		d2.Name = name2
		d2.Operator = d1.Operator
		r.Equal(ErrInvalidOperator, m.Upsert(d2))
		d2.Operator = op2
		d2.SelfStakeBucketIdx = d1.SelfStakeBucketIdx
		r.Equal(ErrInvalidSelfStkIndex, m.Upsert(d2))
		d2.SelfStakeBucketIdx = self2
		// upsert(d2)
		d2.Operator = identityset.Address(14 + i*2)
		r.NoError(m.Upsert(d2))
		d2.Name = d2.Operator.String()[:12]
		r.NoError(m.Upsert(d2))

		// new n1 conflict with existing
		n1 := list[(u2+size/2)%size].Clone()
		n1.Owner = identityset.Address(15 + i*2)
		r.Equal(ErrInvalidCanName, m.Upsert(n1))
		n1.Name = name1
		r.Equal(ErrInvalidOperator, m.Upsert(n1))
		n1.Operator = op1
		r.Equal(ErrInvalidSelfStkIndex, m.Upsert(n1))
		n1.SelfStakeBucketIdx = uint64(size)
		// upsert(n1)
		r.NoError(m.Upsert(n1))

		// new n2 conflict with dirty d2
		n2 := d2.Clone()
		n2.Owner = identityset.Address(16 + i*2)
		r.Equal(ErrInvalidCanName, m.Upsert(n2))
		n2.Name = name2
		r.Equal(ErrInvalidOperator, m.Upsert(n2))
		n2.Operator = op2
		r.Equal(ErrInvalidSelfStkIndex, m.Upsert(n2))
		n2.SelfStakeBucketIdx = uint64(size + 1)
		// upsert(n2)
		r.NoError(m.Upsert(n2))

		// verify conflict with n1
		n2 = n1.Clone()
		n2.Owner = identityset.Address(0)
		r.Equal(ErrInvalidCanName, m.Upsert(n2))
		n2.Name = "noconflict"
		r.Equal(ErrInvalidOperator, m.Upsert(n2))
		n2.Operator = identityset.Address(0)
		r.Equal(ErrInvalidSelfStkIndex, m.Upsert(n2))

		// upsert a candidate w/o change is fine
		r.NoError(m.Upsert(conflict))

		// there are 5 changes (2 dirty + 2 new + 1 w/o change)
		var err error
		list, err = testEqualAllCommit(r, m, list, 5, 2)
		r.NoError(err)

		// test candidate that does not exist
		nonExistAddr := identityset.Address(0)
		r.False(m.ContainsOwner(nonExistAddr))
		r.False(m.ContainsOperator(nonExistAddr))
		r.False(m.ContainsName("notexist"))
		r.False(m.ContainsSelfStakingBucket(1000))
	}
}

func TestFixAlias(t *testing.T) {
	r := require.New(t)

	dk := protocol.NewDock()
	view := protocol.View{}

	for _, fixAlias := range []bool{false, true} {
		// add 6 candidates into cand center
		m, err := NewCandidateCenter(nil, FixAliasOption(fixAlias))
		r.NoError(err)
		for i, v := range testCandidates {
			r.NoError(m.Upsert(testCandidates[i].d))
			r.True(m.ContainsName(v.d.Name))
			r.True(m.ContainsOperator(v.d.Operator))
			r.Equal(v.d, m.GetByName(v.d.Name))
		}
		r.NoError(m.Commit())
		r.NoError(view.Write(_protocolID, m))

		// simulate handleCandidateUpdate: update name
		center := candCenterFromNewCandidateStateManager(r, view, dk)
		name := testCandidates[0].d.Name
		nameAlias := center.GetByName(name)
		nameAlias.Equal(testCandidates[0].d)
		nameAlias.Name = "break"
		{
			r.NoError(center.Upsert(nameAlias))
			delta := center.Delta()
			r.Equal(1, len(delta))
			r.NoError(dk.Load(_protocolID, _stakingCandCenter, &delta))
		}

		center = candCenterFromNewCandidateStateManager(r, view, dk)
		n := center.GetByName("break")
		n.Equal(nameAlias)
		r.True(center.ContainsName("break"))
		// old name does not exist
		r.Nil(center.GetByName(name))
		r.False(center.ContainsName(name))

		// simulate handleCandidateUpdate: update operator
		op := testCandidates[1].d.Operator
		opAlias := testCandidates[1].d.Clone()
		opAlias.Operator = identityset.Address(17)
		r.True(center.ContainsOperator(op))
		r.False(center.ContainsOperator(opAlias.Operator))
		{
			r.NoError(center.Upsert(opAlias))
			delta := center.Delta()
			r.Equal(2, len(delta))
			r.NoError(dk.Load(_protocolID, _stakingCandCenter, &delta))
		}

		// verify cand center with name/op alias
		center = candCenterFromNewCandidateStateManager(r, view, dk)
		n = center.GetByName("break")
		n.Equal(nameAlias)
		r.True(center.ContainsName("break"))
		// old name does not exist
		r.Nil(center.GetByName(name))
		r.False(center.ContainsName(name))
		n = center.GetByOwner(testCandidates[1].d.Owner)
		n.Equal(opAlias)
		r.True(center.ContainsOperator(opAlias.Operator))
		// old operator does not exist
		r.False(center.ContainsOperator(op))

		// cand center Commit()
		{
			r.NoError(center.Commit())
			r.NoError(view.Write(_protocolID, center))
			dk.Reset()
		}

		// verify cand center after Commit()
		center = candCenterFromNewCandidateStateManager(r, view, dk)
		n = center.GetByName("break")
		n.Equal(nameAlias)
		n = center.GetByOwner(testCandidates[1].d.Owner)
		n.Equal(opAlias)
		r.True(center.ContainsOperator(opAlias.Operator))
		if fixAlias {
			r.Nil(center.GetByName(name))
			r.False(center.ContainsName(name))
			r.False(center.ContainsOperator(op))
		} else {
			// alias still exist in name/operator map
			n = center.GetByName(name)
			n.Equal(testCandidates[0].d)
			r.True(center.ContainsName(name))
			r.True(center.ContainsOperator(op))
		}
	}
}

func candCenterFromNewCandidateStateManager(r *require.Assertions, view protocol.View, dk protocol.Dock) *CandidateCenter {
	// get cand center: csm.ConstructBaseView
	v, err := view.Read(_protocolID)
	r.NoError(err)
	center := v.(*CandidateCenter).Base()
	// get changes: csm.Sync()
	delta := CandidateList{}
	err = dk.Unload(_protocolID, _stakingCandCenter, &delta)
	r.True(err == nil || err == protocol.ErrNoName)
	r.NoError(center.SetDelta(delta))
	return center
}
