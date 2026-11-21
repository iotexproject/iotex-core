package bucketguard

import "errors"

// Errors returned by this package. Except for ErrMalformed, each maps to one
// IIP receipt status (§9.5): the six guard statuses 216-221, and
// ErrSponsorMismatch to the existing ErrUnauthorizedOperator (203). The
// staking protocol should do that mapping in one place with errors.Is.
// ErrMalformed marks a stateless failure: the action is invalid and must not
// be included. A counter overflow is ErrGuardMismatch.
var (
	ErrMalformed         = errors.New("malformed bucket guard action")
	ErrBucketGuarded     = errors.New("bucket is guarded")
	ErrBucketNotGuarded  = errors.New("bucket is not guarded")
	ErrGuardMismatch     = errors.New("guard generation, nonce, anchor or deadline mismatch")
	ErrGuardSignature    = errors.New("invalid guard signature")
	ErrGuardScope        = errors.New("bucket cannot be guarded")
	ErrCandidateMismatch = errors.New("candidate does not match expected identifier")
	ErrSponsorMismatch   = errors.New("sender is not the sponsor of the guarded op")
)
