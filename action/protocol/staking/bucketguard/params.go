package bucketguard

import (
	"slices"

	"github.com/iotexproject/iotex-core/v2/pkg/pqcrypto"
)

// Params are the consensus parameters of one version of the rules. They are
// selected by hardfork height: a change (gas calibration, a new algorithm, a
// size limit) is a new ParamsVn activated at a new fork, so that historical
// blocks always replay under the parameters they were produced with.
type Params struct {
	// Algs are the algorithms accepted for new guard keys (bind, rotate,
	// transfer with a new guard).
	Algs []pqcrypto.AlgID
	// MaxBindSize limits the canonical size of BindBucketGuard.
	MaxBindSize int
	// MaxOpSize limits the canonical size of GuardedBucketOp without a new guard.
	MaxOpSize int
	// MaxOpWithNewGuardSize limits the canonical size of GuardedBucketOp with a new guard.
	MaxOpWithNewGuardSize int
	// GasBase, GasPerByte and GasPerVerify price an action as
	// GasBase + GasPerByte*size + k*GasPerVerify, k being the number of
	// signature verifications (IIP §8). GasPerByte equals the staking payload
	// price, so failed actions are no cheaper a data channel than others.
	GasBase      uint64
	GasPerByte   uint64
	GasPerVerify uint64
}

// ParamsV1 returns the parameters of the IIP draft. GasPerVerify is the
// draft value pending linux/amd64 calibration. Never change these values;
// add a ParamsV2 instead.
func ParamsV1() Params {
	return Params{
		Algs:                  []pqcrypto.AlgID{pqcrypto.AlgSLHDSA192s},
		MaxBindSize:           17_000,
		MaxOpSize:             17_000,
		MaxOpWithNewGuardSize: 34_000,
		GasBase:               10_000,
		GasPerByte:            100,
		GasPerVerify:          30_000,
	}
}

func (p Params) accepts(alg pqcrypto.AlgID) bool {
	return alg.Supported() && slices.Contains(p.Algs, alg)
}

// acceptsSigSize reports whether n is the signature size of an accepted
// algorithm. The stateless check cannot know the current guard's algorithm.
func (p Params) acceptsSigSize(n int) bool {
	for _, a := range p.Algs {
		if a.Supported() && a.SignatureSize() == n {
			return true
		}
	}
	return false
}

func (p Params) gas(size int, verifications uint64) uint64 {
	return p.GasBase + p.GasPerByte*uint64(size) + verifications*p.GasPerVerify
}
