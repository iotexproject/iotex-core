// Package bucketguard is the consensus rule set of the IIP draft
// "Quantum-Safe Staking Buckets" (QSB), kept free of state access so that it
// can be pinned by vectors before it is wired into iotex-core.
//
// The package is non-activated: it registers no action, chooses no hardfork
// height and touches no state. Given the current guard record and an action,
// it answers "is this authorized?" and "what is the record afterwards?":
//
//	Validate / IntrinsicGas           stateless checks and pricing (IIP §8, §9.1)
//	AuthorizeBind                     BindBucketGuard steps 4–7 (IIP §5)
//	CheckOp, VerifyOp                 GuardedBucketOp steps 2–3 and 5 (IIP §9.2)
//	Next                              guard record transitions (IIP §9.4)
//	RejectIfGuarded                   legacy staking paths (IIP §10)
//	EncodeCall / EncodeAccept         signing messages for wallets (IIP §7)
//
// The staking protocol stays responsible for state reads and writes, the
// bucket effect of each operation, staking business rules (restake duration,
// candidate-name format), receipts and events. Every rule here is versioned
// through Params; a change of rules is a new Params value behind a new fork.
package bucketguard
