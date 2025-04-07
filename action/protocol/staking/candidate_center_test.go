package staking

import (
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/pkg/unit"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// testEqual verifies m contains exactly the list
func testEqual(m *CandidateCenter, l CandidateList) bool {
	if m.All().Len() != len(l) {
		return false
	}
	for _, v := range l {
		d := m.GetByIdentifier(v.GetIdentifier())
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
		if d == nil && v.isSelfStakeBucketSettled() {
			return false
		}
		if d != nil && !v.Equal(d) {
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
	r.Equal(change, len(m.change.view()))
	// m equal to old list, not equal to current
	r.False(testEqual(m, old))
	r.True(testEqual(m, list))

	// test commit
	r.NoError(m.LegacyCommit())
	r.NoError(m.LegacyCommit()) // commit is idempotent
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
	r.NoError(m.LegacyCommit())
	r.Equal(len(testCandidates), m.Size())
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
	list := m.All()
	r.Equal(len(list), m.Size())
	r.True(testEqual(m, list))
	r.Equal(len(testCandidates)+4, m.Size())

	r.Equal(len(list), m.Size())
	r.True(testEqual(m, list))
	r.NoError(m.LegacyCommit())
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
		r.Equal(action.ErrInvalidCanName, m.Upsert(d1))
		d1.Name = name1
		d1.Operator = conflict.Operator
		r.Equal(ErrInvalidOperator, m.Upsert(d1))
		d1.Operator = op1
		d1.SelfStakeBucketIdx = conflict.SelfStakeBucketIdx
		r.Equal(ErrInvalidSelfStkIndex, m.Upsert(d1))
		d1.SelfStakeBucketIdx = self1

		// upsert(d1)
		d1.Operator = identityset.Address(13 + i*2) // identityset[13]+ don't conflict with existing
		r.NoError(m.Upsert(d1))
		d1.Name = d1.Operator.String()[:12]
		r.NoError(m.Upsert(d1))

		// d2 conflict d1
		d2.Name = d1.Name
		r.Equal(action.ErrInvalidCanName, m.Upsert(d2))
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
		r.Equal(action.ErrInvalidCanName, m.Upsert(n1))
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
		r.Equal(action.ErrInvalidCanName, m.Upsert(n2))
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
		r.Equal(action.ErrInvalidCanName, m.Upsert(n2))
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

	views := protocol.NewViews()

	for _, hasAlias := range []bool{false, true} {
		// add 6 candidates into cand center
		m, err := NewCandidateCenter(nil)
		r.NoError(err)
		for i, v := range testCandidates {
			r.NoError(m.Upsert(testCandidates[i].d))
			r.True(m.ContainsName(v.d.Name))
			r.True(m.ContainsOperator(v.d.Operator))
			r.Equal(v.d, m.GetByName(v.d.Name))
		}
		if hasAlias {
			r.NoError(m.LegacyCommit())
		} else {
			r.NoError(m.Commit())
		}
		views.Write(_protocolID, &ViewData{
			candCenter: m,
		})

		// simulate handleCandidateUpdate: update name
		center := candCenterFromNewCandidateStateManager(r, views)
		name := testCandidates[0].d.Name
		nameAlias := center.GetByName(name)
		nameAlias.Equal(testCandidates[0].d)
		nameAlias.Name = "break"
		r.NoError(center.Upsert(nameAlias))

		center = candCenterFromNewCandidateStateManager(r, views)
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
		}

		// verify cand center with name/op alias
		center = candCenterFromNewCandidateStateManager(r, views)
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
			if hasAlias {
				r.NoError(center.LegacyCommit())
			} else {
				r.NoError(center.Commit())
			}
			views.Write(_protocolID, &ViewData{
				candCenter: center,
			})
		}

		// verify cand center after Commit()
		center = candCenterFromNewCandidateStateManager(r, views)
		n = center.GetByName("break")
		n.Equal(nameAlias)
		n = center.GetByOwner(testCandidates[1].d.Owner)
		n.Equal(opAlias)
		r.True(center.ContainsOperator(opAlias.Operator))
		if !hasAlias {
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

func TestMultipleNonStakingCandidate(t *testing.T) {
	r := require.New(t)

	candStaked := &Candidate{
		Owner:              identityset.Address(1),
		Operator:           identityset.Address(11),
		Reward:             identityset.Address(3),
		Name:               "self-staked",
		Votes:              unit.ConvertIotxToRau(1200000),
		SelfStakeBucketIdx: 1,
		SelfStake:          unit.ConvertIotxToRau(1200000),
	}
	candNonStaked1 := &Candidate{
		Owner:              identityset.Address(2),
		Operator:           identityset.Address(12),
		Reward:             identityset.Address(3),
		Name:               "non-self-staked1",
		Votes:              big.NewInt(0),
		SelfStakeBucketIdx: candidateNoSelfStakeBucketIndex,
		SelfStake:          big.NewInt(0),
	}
	candNonStaked2 := &Candidate{
		Owner:              identityset.Address(3),
		Operator:           identityset.Address(13),
		Reward:             identityset.Address(3),
		Name:               "non-self-staked2",
		Votes:              big.NewInt(0),
		SelfStakeBucketIdx: candidateNoSelfStakeBucketIndex,
		SelfStake:          big.NewInt(0),
	}
	candNonStaked1ColOwner := &Candidate{
		Owner:              identityset.Address(2),
		Operator:           identityset.Address(22),
		Reward:             identityset.Address(3),
		Name:               "non-self-staked1-col-owner",
		Votes:              big.NewInt(0),
		SelfStakeBucketIdx: candidateNoSelfStakeBucketIndex,
		SelfStake:          big.NewInt(0),
	}
	candNonStaked1ColOpt := &Candidate{
		Owner:              identityset.Address(20),
		Operator:           identityset.Address(12),
		Reward:             identityset.Address(3),
		Name:               "non-self-staked1-col-opt",
		Votes:              big.NewInt(0),
		SelfStakeBucketIdx: candidateNoSelfStakeBucketIndex,
		SelfStake:          big.NewInt(0),
	}
	candNonStaked1ColName := &Candidate{
		Owner:              identityset.Address(21),
		Operator:           identityset.Address(23),
		Reward:             identityset.Address(3),
		Name:               "non-self-staked1",
		Votes:              big.NewInt(0),
		SelfStakeBucketIdx: candidateNoSelfStakeBucketIndex,
		SelfStake:          big.NewInt(0),
	}
	candStakedColBucket := &Candidate{
		Owner:              identityset.Address(4),
		Operator:           identityset.Address(14),
		Reward:             identityset.Address(3),
		Name:               "self-staked-col-bucket",
		Votes:              unit.ConvertIotxToRau(1200000),
		SelfStakeBucketIdx: 1,
		SelfStake:          unit.ConvertIotxToRau(1200000),
	}

	checkCandidates := func(candcenter *CandidateCenter, cands []*Candidate) {
		r.True(testEqual(candcenter, CandidateList(cands)))
		// commit
		r.NoError(candcenter.Commit())
		r.True(testEqual(candcenter, CandidateList(cands)))
		// from state manager
		views := protocol.NewViews()
		views.Write(_protocolID, &ViewData{
			candCenter: candcenter,
		})
		candcenter = candCenterFromNewCandidateStateManager(r, views)
		r.True(testEqual(candcenter, CandidateList(cands)))
	}
	t.Run("nonstaked candidate not collision on bucket", func(t *testing.T) {
		candcenter, err := NewCandidateCenter(nil)
		r.NoError(err)
		r.NoError(candcenter.Upsert(candNonStaked1))
		r.NoError(candcenter.Upsert(candNonStaked2))
		checkCandidates(candcenter, []*Candidate{candNonStaked1, candNonStaked2})
	})
	t.Run("staked candidate collision on bucket", func(t *testing.T) {
		candcenter, err := NewCandidateCenter(nil)
		r.NoError(err)
		r.NoError(candcenter.Upsert(candStaked))
		r.ErrorIs(candcenter.Upsert(candStakedColBucket), ErrInvalidSelfStkIndex)
		checkCandidates(candcenter, []*Candidate{candStaked})
	})
	t.Run("nonstaked candidate collision on operator", func(t *testing.T) {
		candcenter, err := NewCandidateCenter(nil)
		r.NoError(err)
		r.NoError(candcenter.Upsert(candNonStaked1))
		r.ErrorIs(candcenter.Upsert(candNonStaked1ColOpt), ErrInvalidOperator)
		checkCandidates(candcenter, []*Candidate{candNonStaked1})
	})
	t.Run("nonstaked candidate collision on name", func(t *testing.T) {
		candcenter, err := NewCandidateCenter(nil)
		r.NoError(err)
		r.NoError(candcenter.Upsert(candNonStaked1))
		r.ErrorIs(candcenter.Upsert(candNonStaked1ColName), action.ErrInvalidCanName)
		checkCandidates(candcenter, []*Candidate{candNonStaked1})
	})
	t.Run("nonstaked candidate update on owner", func(t *testing.T) {
		candcenter, err := NewCandidateCenter(nil)
		r.NoError(err)
		r.NoError(candcenter.Upsert(candNonStaked1))
		r.NoError(candcenter.Upsert(candNonStaked1ColOwner))
		checkCandidates(candcenter, []*Candidate{candNonStaked1ColOwner})
	})
	t.Run("change bucket", func(t *testing.T) {
		candcenter, err := NewCandidateCenter(nil)
		r.NoError(err)
		r.NoError(candcenter.Upsert(candNonStaked1))
		// settle self-stake bucket
		candStaked := candNonStaked1.Clone()
		candStaked.SelfStakeBucketIdx = 1
		candStaked.SelfStake = unit.ConvertIotxToRau(1200000)
		r.NoError(candcenter.Upsert(candStaked))
		// change self-stake bucket
		candUpdated := candStaked.Clone()
		candUpdated.SelfStakeBucketIdx = 2
		r.NoError(candcenter.Upsert(candUpdated))
		checkCandidates(candcenter, []*Candidate{candUpdated})
	})
}

func candCenterFromNewCandidateStateManager(r *require.Assertions, views *protocol.Views) *CandidateCenter {
	// get cand center: csm.ConstructBaseView
	v, err := views.Read(_protocolID)
	r.NoError(err)
	return v.(*ViewData).candCenter
}

func TestCandidateUpsert(t *testing.T) {
	r := require.New(t)

	m, err := NewCandidateCenter(nil)
	r.NoError(err)
	tests := []*Candidate{}
	for _, v := range testCandidates {
		tests = append(tests, v.d.Clone())
	}
	tests[0].Identifier = identityset.Address(10)
	tests[1].Identifier = identityset.Address(11)
	tests[2].Identifier = identityset.Address(12)
	for _, v := range tests {
		r.NoError(m.Upsert(v))
		r.True(m.ContainsName(v.Name))
		r.Equal(v, m.GetByName(v.Name))
	}
	r.Equal(len(tests), m.Size())

	// test export changes and commit
	r.NoError(m.LegacyCommit())
	r.Equal(len(tests), m.Size())
	old := m.All()
	r.True(testEqual(m, old))

	// test existence
	for _, v := range tests {
		r.True(m.ContainsName(v.Name))
		r.True(m.ContainsOwner(v.Owner))
		r.True(m.ContainsOperator(v.Operator))
		r.True(m.ContainsSelfStakingBucket(v.SelfStakeBucketIdx))
		r.Equal(v, m.GetByName(v.Name))
		r.Equal(v, m.GetByOwner(v.Owner))
		r.Equal(v, m.GetBySelfStakingIndex(v.SelfStakeBucketIdx))
	}
	t.Run("name change to another candidate", func(t *testing.T) {
		nameChange := tests[0].Clone()
		nameChange.Name = tests[1].Name
		r.Equal(action.ErrInvalidCanName, m.Upsert(nameChange))
	})

	t.Run("owner change to another candidate", func(t *testing.T) {
		ownerChange := tests[0].Clone()
		ownerChange.Owner = tests[1].Owner
		r.Equal(ErrInvalidOwner, m.Upsert(ownerChange))
	})
	t.Run("operator change to another candidate", func(t *testing.T) {
		operatorChange := tests[0].Clone()
		operatorChange.Operator = tests[1].Operator
		r.Equal(ErrInvalidOperator, m.Upsert(operatorChange))
	})
	t.Run("upsert the same candidate", func(t *testing.T) {
		r.NoError(m.Upsert(tests[0]))
		r.NoError(m.Commit())
		r.Equal(len(tests), m.Size())
		testEqual(m, old)
	})

	t.Run("self staking bucket index change to another candidate", func(t *testing.T) {
		selfStakingBucketChange := tests[0].Clone()
		selfStakingBucketChange.SelfStakeBucketIdx = tests[1].SelfStakeBucketIdx
		r.Equal(ErrInvalidSelfStkIndex, m.Upsert(selfStakingBucketChange))
	})
	t.Run("owner change to non-exist candidate", func(t *testing.T) {
		ownerChange := tests[0].Clone()
		ownerChange.Owner = identityset.Address(28)
		r.NoError(m.Upsert(ownerChange))
		r.NoError(m.Commit())
		r.Equal(ownerChange, m.GetByName(ownerChange.Name))
		r.Equal(ownerChange, m.GetByOwner(ownerChange.Owner))
		r.Equal(ownerChange, m.GetBySelfStakingIndex(ownerChange.SelfStakeBucketIdx))
		r.Equal(ownerChange, m.GetByIdentifier(ownerChange.Identifier))
		r.Equal(len(testCandidates), m.Size())
	})
	t.Run("owner change to another candidate identifier", func(t *testing.T) {
		ownerChange := tests[3].Clone()
		ownerChange.Owner = tests[2].Identifier
		r.Equal(action.ErrInvalidCanName, m.Upsert(ownerChange))
	})
	t.Run("insert new candidate and then change owner", func(t *testing.T) {
		m, err := NewCandidateCenter(nil)
		r.NoError(err)
		for _, v := range tests {
			r.NoError(m.Upsert(v))
		}
		newCandidate := &Candidate{
			Name:               "newCandidate1",
			Owner:              identityset.Address2(100),
			Operator:           identityset.Address2(101),
			Reward:             identityset.Address2(102),
			SelfStake:          big.NewInt(0),
			SelfStakeBucketIdx: 103,
			Votes:              big.NewInt(0),
		}
		r.NoError(m.Upsert(newCandidate))
		cand := newCandidate.Clone()
		cand.Identifier = identityset.Address2(100)
		cand.Owner = identityset.Address2(104)
		r.NoError(m.Upsert(cand))
		r.NoError(m.Commit())
		r.Equal(len(tests)+1, m.Size())
		r.Equal(cand, m.GetByIdentifier(cand.GetIdentifier()))
	})
}
