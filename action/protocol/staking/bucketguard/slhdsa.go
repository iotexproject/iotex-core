package bucketguard

import (
	"github.com/cloudflare/circl/sign/slhdsa"
)

// Verify reports whether sig is a valid pure SLH-DSA-SHA2-192s signature over
// message under the fixed QSB context. publicKey and sig must already be at
// their fixed sizes; malformed inputs return false.
func Verify(publicKey, message, sig []byte) bool {
	if len(publicKey) != PubKeySize || len(sig) != SigSize {
		return false
	}
	var pk slhdsa.PublicKey
	pk.ID = slhdsa.SHA2_192s
	if err := pk.UnmarshalBinary(publicKey); err != nil {
		return false
	}
	return slhdsa.Verify(&pk, slhdsa.NewMessage(message), sig, []byte(Context))
}
