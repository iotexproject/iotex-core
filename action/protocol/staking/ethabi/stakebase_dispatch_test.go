package ethabi

import (
	"encoding/hex"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestBuildReadStateRequestDispatch verifies that BuildReadStateRequest walks the
// version handlers (v1 -> v2 -> v3 -> v4) and routes a call to the first version
// whose method selector matches, rather than stopping at v1.
func TestBuildReadStateRequestDispatch(t *testing.T) {
	for _, c := range []struct {
		name    string
		data    string // selector + args
		ctxType string
	}{
		{
			// bucketsByCandidate is a v1-only selector
			name:    "v1",
			data:    "387c001b000000000000000000000000000000000000000000000000000000000000006000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001000000000000000000000000000000000000000000000000000000000000000568656c6c6f000000000000000000000000000000000000000000000000000000",
			ctxType: "*common.BucketsByCandidateStateContext",
		},
		{
			// contractStakeBucketTypes is a v2-only selector; v1 must reject and fall through
			name:    "v2",
			data:    "017619d40000000000000000000000000000000000000000000000000000000000000064",
			ctxType: "*v2.ContractBucketTypesStateContext",
		},
		{
			// candidateByID is a v3-only selector; v1 and v2 must reject and fall through
			name:    "v3",
			data:    "794368820000000000000000000000000000000000000000000000000000000000000001",
			ctxType: "*common.CandidateByAddressStateContext",
		},
		{
			// candidateDeactivation(address) is a v4-only selector; v1..v3 must reject
			name:    "v4",
			data:    "a1c7820a0000000000000000000000000000000000000000000000000000000000000001",
			ctxType: "*v4.candidateDeactivationStateContext",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := require.New(t)
			data, err := hex.DecodeString(c.data)
			r.NoError(err)
			req, err := BuildReadStateRequest(data)
			r.NoError(err)
			r.NotNil(req)
			r.Equal(c.ctxType, reflect.TypeOf(req).String())
		})
	}
}
