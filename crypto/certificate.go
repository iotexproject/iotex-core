package crypto

import (
	"errors"
)

// Signable defines an interface which would generate a unique hash based on its content
type Signable interface {
	Hash() []byte
}

// PublicKey defines an interface of Public Key
type PublicKey interface {
	Bytes() []byte
	Hex() string
}

// Signature defines an interface of
type Signature interface {
	Bytes() []byte
}

// Verifier defines an interface which could verify a signature
type Verifier interface {
	Verify(signable Signable, signature Signature) bool
	PublicKey() PublicKey
}

// Signer defines an interface which could sign a signable
type Signer interface {
	PublicKey() PublicKey
	Sign(Signable) (Signature, error)
}

// Certificate represents a certificate for a signable
type Certificate struct {
	signable   Signable
	endorsers  []PublicKey
	signatures map[string]Signature
}

// NewCertificate returns a certificate draft given a signable object
func NewCertificate(signable Signable) (*Certificate, error) {
	if signable == nil {
		return nil, errors.New("Cannot create a certificate with signable as nil")
	}
	return &Certificate{
		signable:   signable,
		endorsers:  []PublicKey{},
		signatures: make(map[string]Signature),
	}, nil
}

// Content returns the signable content to ceritificate
func (c *Certificate) Content() Signable {
	return c.signable
}

// Endorse adds the endorser's signature to the certificate
func (c *Certificate) Endorse(endorser Signer) error {
	signature, err := endorser.Sign(c.signable)
	if err != nil {
		return err
	}
	pk := endorser.PublicKey()
	if _, ok := c.signatures[pk.Hex()]; ok {
		return errors.New("duplicate endorser")
	}
	c.endorsers = append(c.endorsers, pk)
	c.signatures[pk.Hex()] = signature

	return nil
}

// Endorsers returns the list of endorsers of this certificate
func (c *Certificate) Endorsers() []PublicKey {
	return c.endorsers
}

// Signature returns the signature of the verifier saved in this certificate
func (c *Certificate) Signature(pk PublicKey) (Signature, bool) {
	if pk == nil {
		return nil, false
	}
	sig, ok := c.signatures[pk.Hex()]

	return sig, ok
}

// VerifiedBy checks whether the certificate contains verifiers' signatures
func (c *Certificate) VerifiedBy(verifiers ...Verifier) bool {
	for _, verifier := range verifiers {
		pkey := verifier.PublicKey()
		signature, ok := c.Signature(pkey)
		if !ok {
			return false
		}
		if !verifier.Verify(c.signable, signature) {
			return false
		}
	}

	return true
}
