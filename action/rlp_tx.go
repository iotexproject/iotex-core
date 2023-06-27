package action

import (
	"encoding/hex"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"golang.org/x/crypto/sha3"
)

func rlpRawHash(rawTx *types.Transaction, chainID uint32, encoding iotextypes.Encoding) hash.Hash256 {
	var h common.Hash
	if encoding == 2 {
		h = types.HomesteadSigner{}.Hash(rawTx)
	} else {
		h = types.NewEIP155Signer(big.NewInt(int64(chainID))).Hash(rawTx)
	}
	return hash.BytesToHash256(h[:])
}

func rlpSignedHash(tx *types.Transaction, chainID uint32, encoding iotextypes.Encoding, sig []byte) (hash.Hash256, error) {
	if len(sig) != 65 {
		return hash.ZeroHash256, errors.Errorf("invalid signature length = %d, expecting 65", len(sig))
	}
	sc := make([]byte, 65)
	copy(sc, sig)
	if sc[64] >= 27 {
		sc[64] -= 27
	}

	var signer types.Signer
	if encoding == 2 {
		signer = types.HomesteadSigner{}
	} else {
		signer = types.NewEIP155Signer(big.NewInt(int64(chainID)))
	}
	signedTx, err := tx.WithSignature(signer, sc)
	if err != nil {
		return hash.ZeroHash256, err
	}

	h := sha3.NewLegacyKeccak256()
	if err = rlp.Encode(h, signedTx); err != nil {
		return hash.ZeroHash256, err
	}
	return hash.BytesToHash256(h.Sum(nil)), nil
}

// DecodeRawTx decodes raw data string into eth tx
func DecodeRawTx(rawData string, chainID uint32) (tx *types.Transaction, sig []byte, pubkey crypto.PublicKey, err error) {
	//remove Hex prefix and decode string to byte
	rawData = strings.Replace(rawData, "0x", "", -1)
	rawData = strings.Replace(rawData, "0X", "", -1)
	var dataInString []byte
	dataInString, err = hex.DecodeString(rawData)
	if err != nil {
		return
	}

	// decode raw data into rlp tx
	tx = &types.Transaction{}
	err = rlp.DecodeBytes(dataInString, tx)
	if err != nil {
		return
	}

	// extract signature and recover pubkey
	var (
		v, r, s = tx.RawSignatureValues()
		recID   = uint32(v.Int64())
		rawHash common.Hash
	)
	if tx.Protected() {
		// https://eips.ethereum.org/EIPS/eip-155
		// for post EIP-155 tx, v is set to {0,1} + CHAIN_ID * 2 + 35
		// convert it to the canonical value {0,1} + 27
		recID -= chainID*2 + 8
		rawHash = types.NewEIP155Signer(big.NewInt(int64(chainID))).Hash(tx)
	} else {
		rawHash = types.HomesteadSigner{}.Hash(tx)
	}
	sig = make([]byte, 64, 65)
	rSize := len(r.Bytes())
	copy(sig[32-rSize:32], r.Bytes())
	sSize := len(s.Bytes())
	copy(sig[64-sSize:], s.Bytes())
	sig = append(sig, byte(recID))

	// recover public key
	pubkey, err = crypto.RecoverPubkey(rawHash[:], sig)
	return
}
