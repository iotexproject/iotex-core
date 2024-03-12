package staking

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEndorsementStatus_Contain(t *testing.T) {
	r := require.New(t)
	cases := []struct {
		status   EndorsementStatus
		contains EndorsementStatus
		expected bool
	}{
		{EndorseExpired, EndorseExpired, true},
		{EndorseExpired, UnEndorsing, false},
		{EndorseExpired, Endorsed, false},
		{EndorseExpired, WithoutEndorsement, false},
		{UnEndorsing, UnEndorsing, true},
		{UnEndorsing, Endorsed, false},
		{UnEndorsing, WithoutEndorsement, false},
		{UnEndorsing, EndorseExpired, false},
		{Endorsed, Endorsed, true},
		{Endorsed, WithoutEndorsement, false},
		{Endorsed, EndorseExpired, false},
		{Endorsed, UnEndorsing, false},
		{WithoutEndorsement, WithoutEndorsement, true},
		{WithoutEndorsement, EndorseExpired, false},
		{WithoutEndorsement, UnEndorsing, false},
		{WithoutEndorsement, Endorsed, false},
		// multiple status
		{EndorseExpired | UnEndorsing, EndorseExpired, true},
		{EndorseExpired | UnEndorsing, UnEndorsing, true},
		{EndorseExpired | UnEndorsing, Endorsed, false},
		{EndorseExpired | UnEndorsing, WithoutEndorsement, false},
	}
	for i, v := range cases {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			r.Equal(v.expected, v.status.Contain(v.contains))
		})
	}
}
