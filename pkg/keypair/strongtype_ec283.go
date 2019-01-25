package keypair

import "github.com/pkg/errors"

const (
	ec283pubKeyLength  = 72
	ec283privKeyLength = 36
)

var (
	// ZeroPublicKey is an instance of PublicKey consisting of all zeros
	ZeroPublicKey PublicKey
	// ZeroPrivateKey is an instance of PrivateKey consisting of all zeros
	ZeroPrivateKey PrivateKey
	// ErrPublicKey indicates the error of public key
	ErrPublicKey = errors.New("invalid public key")
	// ErrPrivateKey indicates the error of private key
	ErrPrivateKey = errors.New("invalid private key")
)

type (
	// PublicKey indicates the type of 72 byte public key generated by EC283 crypto library
	PublicKey [ec283pubKeyLength]byte
	// PrivateKey indicates the type 36 byte public key generated by EC283 library
	PrivateKey [ec283privKeyLength]byte
)

// BytesToEC283PublicKey converts a byte slice to PublicKey
func BytesToEC283PublicKey(pubKey []byte) (PublicKey, error) {
	if len(pubKey) != ec283pubKeyLength {
		return ZeroPublicKey, errors.Wrap(ErrPublicKey, "Invalid public key length")
	}
	var publicKey PublicKey
	copy(publicKey[:], pubKey)
	return publicKey, nil
}

// BytesToEC283PrivateKey converts a byte slice to PrivateKey
func BytesToEC283PrivateKey(priKey []byte) (PrivateKey, error) {
	if len(priKey) != ec283privKeyLength {
		return ZeroPrivateKey, errors.Wrap(ErrPrivateKey, "Invalid private key length")
	}
	var privateKey PrivateKey
	copy(privateKey[:], priKey)
	return privateKey, nil
}
