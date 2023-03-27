package testpr

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"fmt"
)

var RSA_PRIVATE = ''

var RSA_PUBLIC = ''


// Sign sign message with sha256
func Sign(message string, priv *rsa.PrivateKey) (string, error) {
	h := crypto.SHA256.New()
	h.Write([]byte(message))
	hashed := h.Sum(nil)

	sign, err := rsa.SignPKCS1v15(rand.Reader, priv, crypto.SHA256, hashed)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(sign), err
}

// Verify verify message
func Verify(message, sign string, pub *rsa.PublicKey) error {
	h := crypto.SHA256.New()
	h.Write([]byte(message))
	hashed := h.Sum(nil)

	dst := make([]byte, base64.StdEncoding.DecodedLen(len(sign)))
	n, err := base64.StdEncoding.Decode(dst, []byte(sign))
	if err != nil {
		return err
	}

	return rsa.VerifyPKCS1v15(pub, crypto.SHA256, hashed, dst[:n])
}

// LoadPrivateKey load private key by pem
func LoadPrivateKey(pem string) (*rsa.PrivateKey, error) {
	dst, err := base64.StdEncoding.DecodeString(pem)
	if err != nil {
		return nil, err
	}
	key, err := x509.ParsePKCS8PrivateKey(dst)
	if err != nil {
		return nil, err
	}
	return key.(*rsa.PrivateKey), nil
}

// LoadPublicKey load public key by pem
func LoadPublicKey(pem string) (*rsa.PublicKey, error) {
	dst, err := base64.StdEncoding.DecodeString(pem)
	if err != nil {
		return nil, err
	}
	pub, err := x509.ParsePKIXPublicKey(dst)
	if err != nil {
		return nil, err
	}
	return pub.(*rsa.PublicKey), nil
}

func main() {
	dst, err := base64.StdEncoding.DecodeString(RSA_PRIVATE)
	if err != nil {
		fmt.Printf("%s\n", err)
	}
	key, err := x509.ParsePKCS8PrivateKey(dst)
	if err != nil {
		fmt.Printf("%s\n", err)
	}
	fmt.Printf("private key: %s\n", key.(*rsa.PrivateKey))
}