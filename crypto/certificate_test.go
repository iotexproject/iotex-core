package crypto

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type dummySignable struct {
	content string
}

func (d *dummySignable) Hash() []byte {
	return []byte(d.content)
}

type dummyPublicKey struct {
	content string
}

func (d *dummyPublicKey) Bytes() []byte {
	return []byte(d.content)
}

func (d *dummyPublicKey) Hex() string {
	return d.content
}

type dummySignature struct {
	signature string
}

func (d *dummySignature) Bytes() []byte {
	return []byte(d.signature)
}

type dummyVerifier struct {
	secret string
}

func (d *dummyVerifier) PublicKey() PublicKey {
	return &dummyPublicKey{
		d.secret,
	}
}

func (d *dummyVerifier) Verify(signable Signable, signature Signature) bool {
	return d.secret+string(signable.Hash()) == string(signature.Bytes())
}

type dummySigner struct {
	dummyVerifier
}

func (d *dummySigner) Sign(s Signable) (Signature, error) {
	if d.secret == "" {
		return nil, errors.New("fail signature")
	}
	return &dummySignature{
		d.secret + string(s.Hash()),
	}, nil
}

func TestCertificate(t *testing.T) {
	content := "dummy"
	require := require.New(t)
	certificate, err := NewCertificate(nil)
	require.Error(err)
	require.Nil(certificate)
	ds := &dummySignable{
		content,
	}
	certificate, err = NewCertificate(ds)
	require.NoError(err)
	require.NotNil(certificate)

	hos := certificate.Content().Hash()
	require.Equal(content, string(hos))

	secret := ""
	endorser := &dummySigner{
		dummyVerifier{
			secret,
		},
	}
	require.Equal(secret, endorser.PublicKey().Hex())
	err = certificate.Endorse(endorser)
	require.Error(err)

	secret = "secret"
	secret2 := "secret2"
	endorser = &dummySigner{
		dummyVerifier{
			secret,
		},
	}
	endorser2 := &dummySigner{
		dummyVerifier{
			secret2,
		},
	}
	require.Equal(secret, endorser.PublicKey().Hex())
	err = certificate.Endorse(endorser)
	require.NoError(err)
	endorsers := certificate.Endorsers()
	require.Equal(1, len(endorsers))
	require.Equal(secret, endorsers[0].Hex())
	signature, ok := certificate.Signature(endorsers[0])
	require.True(ok)
	require.Equal(secret+content, string(signature.Bytes()))

	r := certificate.VerifiedBy(endorser)
	require.Equal(true, r)
	r = certificate.VerifiedBy(endorser2)
	require.Equal(false, r)
	r = certificate.VerifiedBy([]Verifier{endorser, endorser2}...)
	require.Equal(false, r)

	err = certificate.Endorse(endorser2)
	require.NoError(err)
	r = certificate.VerifiedBy([]Verifier{endorser, endorser2}...)
	require.Equal(true, r)
	r = certificate.VerifiedBy([]Verifier{}...)
	require.Equal(true, r)
}
