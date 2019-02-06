// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package crypto

//#include "lib/blslib/dkg.h"
//#include "lib/blslib/random.h"
//#cgo darwin LDFLAGS: -L${SRCDIR}/lib/blslib -ltblsmnt_macos
//#cgo linux LDFLAGS: -L${SRCDIR}/lib/blslib -ltblsmnt_ubuntu
import "C"
import "errors"

// DKG represents a dkg struct singleton that contains the set of cryptography functions
var DKG dkg

type dkg struct {
}

// KeyPairGeneration generates a dkg key pair
func (d *dkg) KeyPairGeneration(shares [][]uint32, statusMatrix [][numnodes]bool) ([]byte, []byte, []uint32, error) {
	if len(shares) != numnodes || len(statusMatrix) != numnodes || len(shares[0]) != sigSize {
		return []byte{}, []byte{}, []uint32{}, errors.New("dimension of shares or statusMatrix is incorrect")
	}
	var Qs C.ec160_point_aff
	var Qt C.ec_point_aff_twist
	var sharesSer [numnodes][sigSize]C.uint32_t
	var statusMatrixSer [numnodes][numnodes]C.uint8_t
	var askSer [privkeySize]C.uint32_t
	for i := 0; i < numnodes; i++ {
		for j := 0; j < sigSize; j++ {
			sharesSer[i][j] = (C.uint32_t)(shares[i][j])
		}
		for j := 0; j < numnodes; j++ {
			if statusMatrix[i][j] {
				statusMatrixSer[i][j] = 1
			} else {
				statusMatrixSer[i][j] = 0
			}
		}
	}
	C.dkg_keypair_gen(&sharesSer[0], &statusMatrixSer[0], &askSer[0], &Qs, &Qt)
	s, err := pointSerialization(Qs)
	if err != nil {
		return []byte{}, []byte{}, []uint32{}, err
	}
	t, err := twistPointSerialization(Qt)
	if err != nil {
		return []byte{}, []byte{}, []uint32{}, err
	}
	ask := make([]uint32, privkeySize)
	for i, x := range askSer {
		ask[i] = uint32(x)
	}
	return s, t, ask, nil
}

// SkGeneration generates a secret key
func (d *dkg) SkGeneration() []uint32 {
	var sk [privkeySize]C.uint32_t
	C.dkg_sk_generation(&sk[0])
	result := make([]uint32, len(sk))
	for i, x := range sk {
		result[i] = (uint32)(x)
	}
	return result
}

// Init is the share initialization method using shamir secret sharing
func (d *dkg) Init(ms []uint32, ids [][]uint8) ([][]uint32, [][]uint32, [][]byte, error) {
	if len(ids) != numnodes {
		return [][]uint32{}, [][]uint32{}, [][]byte{}, errors.New("dimension of ids is incorrect")
	}
	var idsSer [numnodes][idlength]C.uint8_t
	var msSer [privkeySize]C.uint32_t
	var shares [numnodes][sigSize]C.uint32_t
	var coeffs [Degree + 1][sigSize]C.uint32_t
	var witnesses [Degree + 1]C.ec160_point_aff
	for i := 0; i < numnodes; i++ {
		for j := 0; j < idlength; j++ {
			idsSer[i][j] = (C.uint8_t)(ids[i][j])
		}
	}
	for i := 0; i < privkeySize; i++ {
		msSer[i] = (C.uint32_t)(ms[i])
	}
	ok := C.dkg_init(&msSer[0], &coeffs[0], &idsSer[0], &shares[0], &witnesses[0])
	if ok == 1 {
		coeffsDes := make([][]uint32, Degree+1)
		for i := 0; i < Degree+1; i++ {
			coeffsDes[i] = make([]uint32, sigSize)
			for j := 0; j < sigSize; j++ {
				coeffsDes[i][j] = (uint32)(coeffs[i][j])
			}
		}
		sharesDes := make([][]uint32, numnodes)
		for i := 0; i < numnodes; i++ {
			sharesDes[i] = make([]uint32, sigSize)
			for j := 0; j < sigSize; j++ {
				sharesDes[i][j] = (uint32)(shares[i][j])
			}
		}
		var witnessByte [][]byte
		for _, point := range witnesses {
			wb, err := pointSerialization(point)
			if err != nil {
				return [][]uint32{}, [][]uint32{}, [][]byte{}, err
			}
			witnessByte = append(witnessByte, wb)
		}

		return coeffsDes, sharesDes, witnessByte, nil
	}
	return [][]uint32{}, [][]uint32{}, [][]byte{}, errors.New("failed to initialize shamir secret sharing")
}

// SharesCollect collects and verifies the received keys
func (d *dkg) SharesCollect(id []uint8, shares [][]uint32, witnesses [][][]byte) ([numnodes]bool, error) {
	if len(shares) != numnodes || len(shares[0]) != sigSize || len(witnesses) != numnodes || len(witnesses[0]) != Degree+1 {
		return [numnodes]bool{}, errors.New("dimension of shares or witnesses is incorrect")
	}
	var idSer [idlength]C.uint8_t
	var sharesSer [numnodes][sigSize]C.uint32_t
	var witnessList [numnodes][Degree + 1]C.ec160_point_aff
	var sharestatus [numnodes]C.uint8_t
	for i := 0; i < numnodes; i++ {
		for j := 0; j < sigSize; j++ {
			sharesSer[i][j] = (C.uint32_t)(shares[i][j])
		}
		for j := 0; j < Degree+1; j++ {
			point, err := pointDeserialization(witnesses[i][j])
			if err != nil {
				return [numnodes]bool{}, errors.New("failed to deserialize point")
			}
			witnessList[i][j] = point
		}
	}
	for i := 0; i < idlength; i++ {
		idSer[i] = (C.uint8_t)(id[i])
	}

	C.dkg_shares_collect(&idSer[0], &sharesSer[0], &witnessList[0], &sharestatus[0])

	var result [numnodes]bool
	for i := 0; i < numnodes; i++ {
		if sharestatus[i] == 1 {
			result[i] = true
		}
	}
	return result, nil
}

// ShareVerify verifies the received secret share
func (d *dkg) ShareVerify(id []uint8, share []uint32, witness [][]byte) (bool, error) {
	if len(share) != sigSize || len(witness) != Degree+1 {
		return false, errors.New("dimension of share or witness is incorrect")
	}
	var idSer [idlength]C.uint8_t
	var shareSer [sigSize]C.uint32_t
	var witnessSer [Degree + 1]C.ec160_point_aff

	for i := 0; i < sigSize; i++ {
		shareSer[i] = (C.uint32_t)(share[i])
	}
	for i := 0; i < Degree+1; i++ {
		point, err := pointDeserialization(witness[i])
		if err != nil {
			return false, errors.New("failed to deserialize point")
		}
		witnessSer[i] = point
	}
	for i := 0; i < idlength; i++ {
		idSer[i] = (C.uint8_t)(id[i])
	}
	result := C.dkg_share_verify(&idSer[0], &shareSer[0], &witnessSer[0])
	if result == 1 {
		return true, nil
	}
	return false, nil
}

// RndGenerate generates a random byte array of IDLENGTH size
func RndGenerate() []uint8 {
	var rnd [idlength]C.uint8_t
	C.rnd_generate(&rnd[0], (C.uint32_t)(idlength))
	result := make([]uint8, len(rnd))
	for i, x := range rnd {
		result[i] = (uint8)(x)
	}
	return result
}
