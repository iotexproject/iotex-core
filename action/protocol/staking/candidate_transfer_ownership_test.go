package staking

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/testutil/testdb"
	"github.com/stretchr/testify/require"
)

func TestCandidateTransferOwnership(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	sm := testdb.NewMockStateManager(ctrl)
	ct := newCandidateTransferOwnership()
	addr1, _ := address.FromString("io10a298zmzvrt4guq79a9f4x7qedj59y7ery84he")
	addr2, _ := address.FromString("io13sj9mzpewn25ymheukte4v39hvjdtrfp00mlyv")
	addr3, _ := address.FromString("io19d0p3ah4g8ww9d7kcxfq87yxe7fnr8rpth5shj")
	tests := []struct {
		name     string
		newOwner address.Address
	}{
		{"test", addr1},
		{"test2", addr2},
		{"test3", addr3},
	}
	for _, test := range tests {
		ct.Update(test.name, test.newOwner)
	}
	data, err := ct.Serialize()
	r.NoError(err)
	ct2 := newCandidateTransferOwnership()
	r.NoError(ct2.Deserialize(data))
	r.Equal(ct.NameToOwner, ct2.NameToOwner)
	ct1 := newCandidateTransferOwnership()
	r.NoError(ct1.LoadFromStateManager(sm))
	r.Empty(ct1.NameToOwner)
	r.NoError(ct.StoreToStateManager(sm))
	r.NoError(ct1.LoadFromStateManager(sm))
	r.NotEmpty(ct1.NameToOwner)
	r.Equal(ct.NameToOwner, ct1.NameToOwner)
}
