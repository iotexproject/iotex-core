package bucketguard

import "github.com/iotexproject/iotex-core/v2/pkg/pqcrypto"

// Verify reports whether sig is a valid pure SLH-DSA-SHA2-192s signature over
// message under the fixed QSB context. Malformed inputs return false.
func Verify(publicKey, message, sig []byte) bool {
	pk, err := pqcrypto.ParsePublicKey(pqcrypto.AlgSLHDSA192s, publicKey)
	if err != nil {
		return false
	}
	return pk.Verify([]byte(Context), message, sig)
}
