// Package bucketguard is a non-activated scaffold for the IIP "Quantum-Safe
// Staking Buckets" draft.
//
// It contains the consensus-neutral pieces that can be reviewed before wiring
// the feature into iotex-core: action types, canonical encoding and gas,
// stateless validation, transcript encoding, and SLH-DSA verification.
//
// This package deliberately does not register an action, choose a hardfork
// height, or mutate staking state. Integration must be fork-gated in
// FeatureCtx, and every legacy staking path that can change a guarded bucket
// must be audited before activation.
package bucketguard
