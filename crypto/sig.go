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

	"github.com/iotexproject/iotex-core/pkg/enc"
	"github.com/iotexproject/iotex-core/pkg/keypair"
)

// NewKeyPair generates a new public/private key pair
func NewKeyPair() (keypair.PublicKey, keypair.PrivateKey, error) {
	var kp C.ec283_key_pair
	C.key_generation(&kp)
	pubKey := kp.Q
	privKey := kp.d
	pub, err := publicKeySerialization(pubKey)
	if err != nil {
		return keypair.ZeroPublicKey, keypair.ZeroPrivateKey, err
	}
	priv, err := privateKeySerialization(privKey)
	if err != nil {
		return keypair.ZeroPublicKey, keypair.ZeroPrivateKey, err
	}

	return pub, priv, nil
}

// Sign signs the msg
func Sign(priv keypair.PrivateKey, msg []byte) []byte {
	msgString := string(msg[:])
	privKey, err := privateKeyDeserialization(priv)
	if err != nil {
		return nil
	}
	var signature C.ecdsa_signature
	C.ECDSA_sign(&privKey[0], (*C.uint8_t)(&msg[0]), (C.uint64_t)(len(msgString)), &signature)
	sign, err := signatureSerialization(signature)
	if err != nil {
		return nil
	}
	return sign
}

// Verify verifies the signature
func Verify(pub keypair.PublicKey, msg []byte, sig []byte) bool {
	msgString := string(msg[:])
	pubKey, err := publicKeyDeserialization(pub)
	if err != nil {
		return false
	}
	signature, err := signatureDeserialization(sig)
	if err != nil {
		return false
	}
	return C.ECDSA_verify(&pubKey, (*C.uint8_t)(&msg[0]), (C.uint64_t)(len(msgString)), &signature) == 1
}

func publicKeySerialization(pubKey C.ec283_point_lambda_aff) (keypair.PublicKey, error) {
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
	return keypair.BytesToPublicKey(buf.Bytes())
}

func publicKeyDeserialization(pubKey keypair.PublicKey) (C.ec283_point_lambda_aff, error) {
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

func privateKeySerialization(privKey [9]C.uint32_t) (keypair.PrivateKey, error) {
	var d [9]uint32
	for i := 0; i < 9; i++ {
		d[i] = (uint32)(privKey[i])
	}
	buf := new(bytes.Buffer)
	err := binary.Write(buf, enc.MachineEndian, d)
	if err != nil {
		return keypair.ZeroPrivateKey, err
	}
	return keypair.BytesToPrivateKey(buf.Bytes())
}

func privateKeyDeserialization(privKey keypair.PrivateKey) ([9]C.uint32_t, error) {
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

func signatureSerialization(signature C.ecdsa_signature) ([]byte, error) {
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

func signatureDeserialization(signatureBytes []byte) (C.ecdsa_signature, error) {
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
