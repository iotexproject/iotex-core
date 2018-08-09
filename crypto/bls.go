// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package crypto

//#include "lib/blslib/bls.h"
//#include "lib/blslib/blskey.h"
//#include "lib/blslib/tbls.h"
//#include "lib/blslib/mnt160.h"
//#cgo darwin LDFLAGS: -L${SRCDIR}/lib/blslib -ltblsmnt_macos
//#cgo linux LDFLAGS: -L${SRCDIR}/lib/blslib -ltblsmnt_ubuntu
import "C"
import (
	"bytes"
	"encoding/binary"

	"github.com/iotexproject/iotex-core/pkg/enc"
	"github.com/pkg/errors"
)

const (
	degree      = 10
	idlength    = 32
	sigSize     = 5 // number of uint32s in sig
	privkeySize = 5
	numnodes    = 21
)

var (
	// BLS represents a bls struct singleton that contains the set of cryptography functions
	BLS bls
	// ErrSignError indicates error for failing to sign
	ErrSignError = errors.New("Could not sign message")
	// ErrKeyGeneration indicates error for failing to generate keys
	ErrKeyGeneration = errors.New("Could not generate keys")
	// ErrInvalidKey indicates error for public key
	ErrInvalidKey = errors.New("Key is invalid")
	// ErrInvalidSignature indicates error for signature
	ErrInvalidSignature = errors.New("Signature is invalid")
)

type bls struct {
}

// NewPubKey generate public key from secret key
func (b *bls) NewPubKey(sk []uint32) ([]byte, error) {
	var Qt C.ec_point_aff_twist
	var skSer [privkeySize]C.uint32_t

	for i := 0; i < privkeySize; i++ {
		skSer[i] = (C.uint32_t)(sk[i])
	}

	ok := C.bls_pk_generation(&skSer[0], &Qt)
	if ok == 1 {
		pk, err := twistPointSerialization(Qt)
		if err != nil {
			return []byte{}, err
		}
		return pk, nil
	}
	return []byte{}, ErrKeyGeneration
}

// Sign signs a message given a private key
func (b *bls) Sign(privkey []uint32, msg []byte) (bool, []byte, error) {
	var sigSer [sigSize]C.uint32_t
	var privkeySer [privkeySize]C.uint32_t
	msgString := string(msg[:])
	for i := 0; i < privkeySize; i++ {
		privkeySer[i] = (C.uint32_t)(privkey[i])
	}

	if ok := C.BLS_sign(&privkeySer[0], (*C.uint8_t)(&msg[0]), (C.uint64_t)(len(msgString)), &sigSer[0]); ok == 1 {
		sig, err := b.signatureSerialization(sigSer)
		if err != nil {
			return false, []byte{}, err
		}
		return true, sig, nil
	}
	return false, []byte{}, ErrSignError
}

// Verify verifies a signature
func (b *bls) Verify(pubkey []byte, msg []byte, signature []byte) error {
	msgString := string(msg[:])
	pk, err := twistPointDeserialization(pubkey)
	if err != nil {
		return err
	}

	sigSer, err := b.signatureDeserialization(signature)
	if err != nil {
		return err
	}

	ok := C.BLS_verify(&pk, (*C.uint8_t)(&msg[0]), (C.uint64_t)(len(msgString)), &sigSer[0])
	if ok == 1 {
		return nil
	}
	return ErrInvalidKey
}

// PkValidation returns whether a public key is valid or not
func (b *bls) PkValidation(pk []byte) error {
	pkSer, err := twistPointDeserialization(pk)
	if err != nil {
		return err
	}

	ok := C.bls_pk_validation(&pkSer)
	if ok == 1 {
		return nil
	}
	return ErrInvalidKey
}

// SignShare signs the message and returns the signature
func (b *bls) SignShare(privkey []uint32, msg []byte) (bool, []byte, error) {
	return b.Sign(privkey, msg)
}

// VerifyShare verifies a signature given a message and a public key
func (b *bls) VerifyShare(pubkey []byte, msg []byte, sig []byte) error {
	return b.Verify(pubkey, msg, sig)
}

// SignAggregate generates an aggregate signature
func (b *bls) SignAggregate(ids [][]uint8, sigs [][]byte) ([]byte, error) {
	var aggsig [sigSize]C.uint32_t
	var idsSer [degree + 1][idlength]C.uint8_t
	var sigsSer [degree + 1][sigSize]C.uint32_t
	var err error

	for i := 0; i < degree+1; i++ {
		for j := 0; j < idlength; j++ {
			idsSer[i][j] = (C.uint8_t)(ids[i][j])
		}
		sigsSer[i], err = b.signatureDeserialization(sigs[i])
		if err != nil {
			return []byte{}, err
		}
	}
	ok := C.TBLS_sign_aggregate(&idsSer[0], &sigsSer[0], &aggsig[0])
	if ok == 1 {
		sig, err := b.signatureSerialization(aggsig)
		if err != nil {
			return []byte{}, err
		}
		return sig, nil
	}
	return []byte{}, errors.Wrap(ErrInvalidSignature, "Failed to generate aggregate signature")
}

// VerifyAggregate verifies the aggregate signature given that there are at least degree+1 signers
func (b *bls) VerifyAggregate(ids [][]uint8, pubkeys [][]byte, msg []byte, aggsig []byte) error {
	var idsSer [degree + 1][idlength]C.uint8_t
	var pubkeysSer [degree + 1]C.ec_point_aff_twist
	var err error
	msgString := string(msg[:])

	for i := 0; i < degree+1; i++ {
		for j := 0; j < idlength; j++ {
			idsSer[i][j] = (C.uint8_t)(ids[i][j])
		}
		pubkeysSer[i], err = twistPointDeserialization(pubkeys[i])
		if err != nil {
			return err
		}
	}
	aggsigSer, err := b.signatureDeserialization(aggsig)
	if err != nil {
		return err
	}

	if ok := C.TBLS_verify_aggregate(&idsSer[0], &pubkeysSer[0], (*C.uint8_t)(&msg[0]), (C.uint64_t)(len(msgString)), &aggsigSer[0]); ok == 1 {
		return nil
	}
	return errors.Wrap(ErrInvalidSignature, "Error when verify aggregate signature")
}

func (b *bls) signatureSerialization(sigSer [sigSize]C.uint32_t) ([]byte, error) {
	var sig [sigSize]uint32
	for i, x := range sigSer {
		sig[i] = (uint32)(x)
	}
	buf := new(bytes.Buffer)
	err := binary.Write(buf, enc.MachineEndian, sig)
	if err != nil {
		return []byte{}, err
	}
	return buf.Bytes(), nil
}

func (b *bls) signatureDeserialization(signature []byte) ([sigSize]C.uint32_t, error) {
	var sig [sigSize]uint32
	buff := bytes.NewReader(signature)
	err := binary.Read(buff, enc.MachineEndian, &sig)
	if err != nil {
		return [sigSize]C.uint32_t{}, err
	}
	var sigSer [sigSize]C.uint32_t
	for i, x := range sig {
		sigSer[i] = (C.uint32_t)(x)
	}
	return sigSer, nil
}

func pointSerialization(point C.ec160_point_aff) ([]byte, error) {
	var xl [10]uint32
	for i := 0; i < 5; i++ {
		xl[i] = (uint32)(point.x[i])
		xl[i+5] = (uint32)(point.y[i])
	}
	buf := new(bytes.Buffer)
	err := binary.Write(buf, enc.MachineEndian, xl)
	if err != nil {
		return []byte{}, err
	}
	return buf.Bytes(), nil
}

func pointDeserialization(point []byte) (C.ec160_point_aff, error) {
	var xl [10]uint32
	var ecPoint C.ec160_point_aff
	rbuf := bytes.NewReader(point[:])
	err := binary.Read(rbuf, enc.MachineEndian, &xl)
	if err != nil {
		return ecPoint, err
	}
	for i := 0; i < 5; i++ {
		ecPoint.x[i] = (C.uint32_t)(xl[i])
		ecPoint.y[i] = (C.uint32_t)(xl[i+5])
	}
	return ecPoint, nil
}

func twistPointSerialization(point C.ec_point_aff_twist) ([]byte, error) {
	var xl [30]uint32
	for i := 0; i < 5; i++ {
		xl[i] = (uint32)(point.x.a0[i])
		xl[i+5] = (uint32)(point.x.a1[i])
		xl[i+10] = (uint32)(point.x.a2[i])
		xl[i+15] = (uint32)(point.y.a0[i])
		xl[i+20] = (uint32)(point.y.a1[i])
		xl[i+25] = (uint32)(point.y.a2[i])
	}
	buf := new(bytes.Buffer)
	err := binary.Write(buf, enc.MachineEndian, xl)
	if err != nil {
		return []byte{}, err
	}
	return buf.Bytes(), nil
}

func twistPointDeserialization(point []byte) (C.ec_point_aff_twist, error) {
	var xl [30]uint32
	var ecPoint C.ec_point_aff_twist
	rbuf := bytes.NewReader(point[:])
	err := binary.Read(rbuf, enc.MachineEndian, &xl)
	if err != nil {
		return ecPoint, err
	}
	for i := 0; i < 5; i++ {
		ecPoint.x.a0[i] = (C.uint32_t)(xl[i])
		ecPoint.x.a1[i] = (C.uint32_t)(xl[i+5])
		ecPoint.x.a2[i] = (C.uint32_t)(xl[i+10])
		ecPoint.y.a0[i] = (C.uint32_t)(xl[i+15])
		ecPoint.y.a1[i] = (C.uint32_t)(xl[i+20])
		ecPoint.y.a2[i] = (C.uint32_t)(xl[i+25])
	}
	return ecPoint, nil
}
