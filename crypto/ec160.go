// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package crypto

//#include "lib/blslib/ecdsa160.h"
//#include "lib/blslib/ecckey160.h"
//#cgo darwin LDFLAGS: -L${SRCDIR}/lib/blslib -ltblsmnt_macos
//#cgo linux LDFLAGS: -L${SRCDIR}/lib/blslib -ltblsmnt_ubuntu
import "C"
import (
	"bytes"
	"encoding/binary"
	"errors"

	"github.com/iotexproject/iotex-core/pkg/enc"
)

// EC160 represents an ec160 struct singleton that contains the set of cryptography functions based on the elliptic
// curve 160. It is used for blockchain address creation, block signature and verification.
var EC160 ec160

type ec160 struct {
}

// NewKeyPair generates a new public/private key pair
func (c *ec160) NewKeyPair() ([]byte, []byte, error) {
	var kp C.ec160_key_pair
	C.keypair160_generation(&kp)
	pubKey := kp.Q
	privKey := kp.d
	pub, err := c.publicKeySerialization(pubKey)
	if err != nil {
		return []byte{}, []byte{}, err
	}
	priv, err := c.privateKeySerialization(privKey)
	if err != nil {
		return []byte{}, []byte{}, err
	}

	return pub, priv, nil
}

// NewPubKey generates a new public key
func (c *ec160) NewPubKey(priv []byte) ([]byte, error) {
	var pubKey C.ec160_point_aff
	privKey, err := c.privateKeyDeserialization(priv)
	if err != nil {
		return []byte{}, nil
	}
	if result := C.ec160_pk_generation(&privKey[0], &pubKey); result == 0 {
		return []byte{}, errors.New("fail to generate the public key")
	}
	pub, err := c.publicKeySerialization(pubKey)
	if err != nil {
		return []byte{}, err
	}
	return pub, nil
}

// Sign signs the msg
func (c *ec160) Sign(priv []byte, msg []byte) []byte {
	msgString := string(msg)
	privKey, err := c.privateKeyDeserialization(priv)
	if err != nil {
		return nil
	}
	var signature C.ecdsa160_signature
	C.ECDSA160_sign(&privKey[0], (*C.uint8_t)(&msg[0]), (C.uint64_t)(len(msgString)), &signature)
	sign, err := c.signatureSerialization(signature)
	if err != nil {
		return nil
	}
	return sign
}

// Verify verifies the signature
func (c *ec160) Verify(pub []byte, msg []byte, sig []byte) bool {
	msgString := string(msg)
	pubKey, err := c.publicKeyDeserialization(pub)
	if err != nil {
		return false
	}
	signature, err := c.signatureDeserialization(sig)
	if err != nil {
		return false
	}
	return C.ECDSA160_verify(&pubKey, (*C.uint8_t)(&msg[0]), (C.uint64_t)(len(msgString)), &signature) == 1
}

func (c *ec160) publicKeySerialization(pubKey C.ec160_point_aff) ([]byte, error) {
	var xy [10]uint32
	for i := 0; i < 5; i++ {
		xy[i] = (uint32)(pubKey.x[i])
		xy[i+5] = (uint32)(pubKey.y[i])
	}
	buf := new(bytes.Buffer)
	err := binary.Write(buf, enc.MachineEndian, xy)
	if err != nil {
		return []byte{}, err
	}
	return buf.Bytes(), nil
}

func (c *ec160) publicKeyDeserialization(pubKey []byte) (C.ec160_point_aff, error) {
	var xy [10]uint32
	var pub C.ec160_point_aff
	rbuf := bytes.NewReader(pubKey)
	err := binary.Read(rbuf, enc.MachineEndian, &xy)
	if err != nil {
		return pub, err
	}
	for i := 0; i < 5; i++ {
		pub.x[i] = (C.uint32_t)(xy[i])
		pub.y[i] = (C.uint32_t)(xy[i+5])
	}
	return pub, nil
}

func (c *ec160) privateKeySerialization(privKey [5]C.uint32_t) ([]byte, error) {
	var d [5]uint32
	for i := 0; i < 5; i++ {
		d[i] = (uint32)(privKey[i])
	}
	buf := new(bytes.Buffer)
	err := binary.Write(buf, enc.MachineEndian, d)
	if err != nil {
		return buf.Bytes(), err
	}
	return buf.Bytes(), nil
}

func (c *ec160) privateKeyDeserialization(privKey []byte) ([5]C.uint32_t, error) {
	var d [5]uint32
	var priv [5]C.uint32_t
	rbuf := bytes.NewReader(privKey)
	err := binary.Read(rbuf, enc.MachineEndian, &d)
	if err != nil {
		return priv, err
	}
	for i := 0; i < 5; i++ {
		priv[i] = (C.uint32_t)(d[i])
	}
	return priv, nil
}

func (c *ec160) signatureSerialization(signature C.ecdsa160_signature) ([]byte, error) {
	var rs [10]uint32
	for i := 0; i < 5; i++ {
		rs[i] = (uint32)(signature.r[i])
		rs[i+5] = (uint32)(signature.s[i])
	}
	buf := new(bytes.Buffer)
	err := binary.Write(buf, enc.MachineEndian, rs)
	if err != nil {
		return buf.Bytes(), err
	}
	return buf.Bytes(), nil
}

func (c *ec160) signatureDeserialization(signatureBytes []byte) (C.ecdsa160_signature, error) {
	var rs [10]uint32
	var signature C.ecdsa160_signature
	buff := bytes.NewReader(signatureBytes)
	err := binary.Read(buff, enc.MachineEndian, &rs)
	if err != nil {
		return signature, err
	}
	for i := 0; i < 5; i++ {
		signature.r[i] = (C.uint32_t)(rs[i])
		signature.s[i] = (C.uint32_t)(rs[i+5])
	}
	return signature, nil
}
