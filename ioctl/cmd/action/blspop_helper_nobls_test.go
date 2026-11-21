package action

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action"
)

// TestRegisterBLSModeMatrix pins the flag classification for the register
// path, including the Zanzibar-era opt-out.
func TestRegisterBLSModeMatrix(t *testing.T) {
	r := require.New(t)

	m, err := (&blsPoPFlags{}).classifyForRegister()
	r.NoError(err)
	r.Equal(blsModeAutoDerive, m, "no flags must keep deriving from the signer")

	m, err = (&blsPoPFlags{noBLS: true}).classifyForRegister()
	r.NoError(err)
	r.Equal(blsModeNone, m, "--no-bls must register without a key")

	_, err = (&blsPoPFlags{noBLS: true, privKeyHex: "aa"}).classifyForRegister()
	r.ErrorContains(err, "--no-bls")

	_, err = (&blsPoPFlags{noBLS: true, pubKeyHex: "aa", popHex: "bb"}).classifyForRegister()
	r.ErrorContains(err, "--no-bls")
}

// TestResolveBLSForRegisterNone checks the opt-out returns no material, which
// is what makes stake2register build the legacy shape.
func TestResolveBLSForRegisterNone(t *testing.T) {
	r := require.New(t)
	pk, pop, err := resolveBLSForRegister(&blsPoPFlags{noBLS: true}, "", "", nil)
	r.NoError(err)
	r.Empty(pk)
	r.Empty(pop)
}

// TestRegisterActionShapeByBLS pins the invariant stake2register relies on:
// the stake rides in value only when a BLS key is present, and the legacy
// constructor is the only way to build a keyless registration. Were the
// WithBLS constructor to accept an empty key it would leave value set and
// amount nil, and Amount() -- which keys off WithBLS() -- would return nil,
// panicking validation on the first Cmp.
func TestRegisterActionShapeByBLS(t *testing.T) {
	r := require.New(t)
	const (
		addr   = "io187wzp08vnhjjpkydnr97qlh8kh0dpkkytfam8j"
		amount = "1200000000000000000000000"
	)

	noBLS, err := action.NewCandidateRegister("cand", addr, addr, addr, amount, 1, true, nil)
	r.NoError(err)
	r.False(noBLS.WithBLS())
	r.NotNil(noBLS.Amount(), "Amount() must be usable without a BLS key")
	r.NotNil(noBLS.LegacyAmount())
	r.Nil(noBLS.Value())

	_, err = action.NewCandidateRegisterWithBLS("cand", addr, addr, addr, amount, 1, true, nil, nil, nil)
	r.Error(err, "the WithBLS constructor must refuse an empty key rather than build the broken shape")
}
