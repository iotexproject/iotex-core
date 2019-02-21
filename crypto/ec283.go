// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package crypto

//#include "lib/ecckey.h"
//#include "lib/ecdsa.h"
//#include "lib/sect283k1.h"
//#include "lib/gfp.h"
//#include "lib/blake256.h"
//#include "lib/schnorr.h"
//#include "lib/random.h"
//#include "lib/gf2283.h"
//#cgo darwin LDFLAGS: -L${SRCDIR}/lib -lsect283k1_macos
//#cgo linux LDFLAGS: -L${SRCDIR}/lib -lsect283k1_ubuntu
import "C"
import (
	"bytes"
	"encoding/binary"
	"errors"

	"github.com/iotexproject/iotex-core/pkg/enc"
	"github.com/iotexproject/iotex-core/pkg/keypair"
)

// EC283 represents an ec283 struct singleton that contains the set of cryptography functions based on the elliptic
// curve 283. It is used for blockchain address creation, action and block signature and verification.
var EC283 ec283

type ec283 struct {
}

// NewKeyPair generates a new public/private key pair
func (c *ec283) NewKeyPair() (keypair.EC283PublicKey, keypair.EC283PrivateKey, error) {
	var kp C.ec283_key_pair
	C.keypair_generation(&kp)
	pubKey := kp.Q
	privKey := kp.d
	pub, err := c.publicKeySerialization(pubKey)
	if err != nil {
		return keypair.ZeroPublicKey, keypair.ZeroPrivateKey, err
	}
	priv, err := c.privateKeySerialization(privKey)
	if err != nil {
		return keypair.ZeroPublicKey, keypair.ZeroPrivateKey, err
	}

	return pub, priv, nil
}

// NewPubKey generates a new public key
func (c *ec283) NewPubKey(priv keypair.EC283PrivateKey) (keypair.EC283PublicKey, error) {
	var pubKey C.ec283_point_lambda_aff
	privKey, err := c.privateKeyDeserialization(priv)
	if err != nil {
		return keypair.ZeroPublicKey, nil
	}
	if result := C.pk_generation(&privKey[0], &pubKey); result == 0 {
		return keypair.ZeroPublicKey, errors.New("fail to generate the public key")
	}
	pub, err := c.publicKeySerialization(pubKey)
	if err != nil {
		return keypair.ZeroPublicKey, err
	}
	return pub, nil
}

// Sign signs the msg
func (c *ec283) Sign(priv keypair.EC283PrivateKey, msg []byte) []byte {
	msgString := string(msg)
	privKey, err := c.privateKeyDeserialization(priv)
	if err != nil {
		return nil
	}
	var signature C.ecdsa_signature
	C.ECDSA_sign(&privKey[0], (*C.uint8_t)(&msg[0]), (C.uint64_t)(len(msgString)), &signature)
	sign, err := c.signatureSerialization(signature)
	if err != nil {
		return nil
	}
	return sign
}

// Verify verifies the signature
func (c *ec283) Verify(pub keypair.EC283PublicKey, msg []byte, sig []byte) bool {
	msgString := string(msg)
	pubKey, err := c.publicKeyDeserialization(pub)
	if err != nil {
		return false
	}
	signature, err := c.signatureDeserialization(sig)
	if err != nil {
		return false
	}
	return C.ECDSA_verify(&pubKey, (*C.uint8_t)(&msg[0]), (C.uint64_t)(len(msgString)), &signature) == 1
}

func (*ec283) publicKeySerialization(pubKey C.ec283_point_lambda_aff) (keypair.EC283PublicKey, error) {
	var xl [18]uint32
	for i := 0; i < 9; i++ {
		xl[i] = (uint32)(pubKey.x[i])
		xl[i+9] = (uint32)(pubKey.l[i])
	}
	buf := new(bytes.Buffer)
	err := binary.Write(buf, enc.MachineEndian, xl)
	if err != nil {
		return keypair.ZeroPublicKey, err
	}
	return keypair.BytesToEC283PublicKey(buf.Bytes())
}

func (*ec283) publicKeyDeserialization(pubKey keypair.EC283PublicKey) (C.ec283_point_lambda_aff, error) {
	var xl [18]uint32
	var pub C.ec283_point_lambda_aff
	rbuf := bytes.NewReader(pubKey[:])
	err := binary.Read(rbuf, enc.MachineEndian, &xl)
	if err != nil {
		return pub, err
	}
	for i := 0; i < 9; i++ {
		pub.x[i] = (C.uint32_t)(xl[i])
		pub.l[i] = (C.uint32_t)(xl[i+9])
	}
	return pub, nil
}

func (*ec283) privateKeySerialization(privKey [9]C.uint32_t) (keypair.EC283PrivateKey, error) {
	var d [9]uint32
	for i := 0; i < 9; i++ {
		d[i] = (uint32)(privKey[i])
	}
	buf := new(bytes.Buffer)
	err := binary.Write(buf, enc.MachineEndian, d)
	if err != nil {
		return keypair.ZeroPrivateKey, err
	}
	return keypair.BytesToEC283PrivateKey(buf.Bytes())
}

func (*ec283) privateKeyDeserialization(privKey keypair.EC283PrivateKey) ([9]C.uint32_t, error) {
	var d [9]uint32
	var priv [9]C.uint32_t
	rbuf := bytes.NewReader(privKey[:])
	err := binary.Read(rbuf, enc.MachineEndian, &d)
	if err != nil {
		return priv, err
	}
	for i := 0; i < 9; i++ {
		priv[i] = (C.uint32_t)(d[i])
	}
	return priv, nil
}

func (*ec283) signatureSerialization(signature C.ecdsa_signature) ([]byte, error) {
	var rs [18]uint32
	for i := 0; i < 9; i++ {
		rs[i] = (uint32)(signature.r[i])
		rs[i+9] = (uint32)(signature.s[i])
	}
	buf := new(bytes.Buffer)
	err := binary.Write(buf, enc.MachineEndian, rs)
	if err != nil {
		return buf.Bytes(), err
	}
	return buf.Bytes(), nil
}

func (*ec283) signatureDeserialization(signatureBytes []byte) (C.ecdsa_signature, error) {
	var rs [18]uint32
	var signature C.ecdsa_signature
	buff := bytes.NewReader(signatureBytes)
	err := binary.Read(buff, enc.MachineEndian, &rs)
	if err != nil {
		return signature, err
	}
	for i := 0; i < 9; i++ {
		signature.r[i] = (C.uint32_t)(rs[i])
		signature.s[i] = (C.uint32_t)(rs[i+9])
	}
	return signature, nil
}
