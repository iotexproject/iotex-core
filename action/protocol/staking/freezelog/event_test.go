// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package freezelog

import (
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// The topic0 an off-chain consumer filters on is derived from EventSignature, so
// the constant and the ABI must not drift apart.
func TestEventSignatureMatchesABI(t *testing.T) {
	r := require.New(t)
	parsed, err := abi.JSON(strings.NewReader(ABIJSON))
	r.NoError(err)
	ev, ok := parsed.Events[EventName]
	r.True(ok)
	r.Equal(EventSignature, ev.Sig)
	r.Equal(crypto.Keccak256Hash([]byte(EventSignature)).Bytes(), ev.ID.Bytes())
}

func TestPackRoundTrip(t *testing.T) {
	r := require.New(t)
	delegate := identityset.Address(1)
	topics, data, err := Pack(EventArgs{
		Era:                  54888,
		Delegate:             delegate,
		FreezeHeight:         46892881,
		BlockCommissionBps:   2000,
		EpochCommissionBps:   2500,
		CommissionConfigured: true,
		TotalWeight:          big.NewInt(2384539),
		SelfStakeBucketIdx:   47,
	})
	r.NoError(err)
	r.Len(topics, 3)

	parsed, err := abi.JSON(strings.NewReader(ABIJSON))
	r.NoError(err)
	ev := parsed.Events[EventName]
	r.Equal(ev.ID.Bytes(), topics[0][:])

	// Indexed uint64 is left-padded into a 32-byte word, as the EVM encodes it.
	r.Equal(uint64(54888), uint64(topics[1][31])|uint64(topics[1][30])<<8|uint64(topics[1][29])<<16)
	r.Equal(delegate.Bytes(), topics[2][12:])

	out, err := ev.Inputs.NonIndexed().Unpack(data)
	r.NoError(err)
	r.Equal(uint64(46892881), out[0])
	r.Equal(uint64(2000), out[1])
	r.Equal(uint64(2500), out[2])
	r.Equal(true, out[3])
	r.Equal(big.NewInt(2384539), out[4])
	r.Equal(uint64(47), out[5])
}

// commissionConfigured is the field that separates "chose to take everything"
// from "never configured anything" -- by value those two are identical.
func TestPackCarriesTheUnconfiguredDistinction(t *testing.T) {
	r := require.New(t)
	parsed, err := abi.JSON(strings.NewReader(ABIJSON))
	r.NoError(err)
	ev := parsed.Events[EventName]

	_, data, err := Pack(EventArgs{
		Era: 54888, Delegate: identityset.Address(1), FreezeHeight: 1,
		BlockCommissionBps: 10000, EpochCommissionBps: 10000,
		CommissionConfigured: false, TotalWeight: big.NewInt(1),
	})
	r.NoError(err)
	out, err := ev.Inputs.NonIndexed().Unpack(data)
	r.NoError(err)
	r.Equal(uint64(10000), out[1])
	r.Equal(false, out[3], "10000bps with configured=false means unconfigured, not a 100% choice")
}

func TestPackRejectsNilDelegateAndToleratesNilWeight(t *testing.T) {
	r := require.New(t)
	_, _, err := Pack(EventArgs{Era: 1})
	r.ErrorIs(err, ErrNilAddress)

	_, _, err = Pack(EventArgs{Era: 1, Delegate: identityset.Address(1)})
	r.NoError(err, "a nil weight encodes as zero rather than failing the freeze")
}
