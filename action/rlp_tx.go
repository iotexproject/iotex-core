package action

import (
	"encoding/hex"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/pkg/errors"
	"golang.org/x/crypto/sha3"
)

func rlpRawHash(rawTx *types.Transaction, chainID uint32) (hash.Hash256, error) {
	h := types.NewEIP155Signer(big.NewInt(int64(chainID))).Hash(rawTx)
	return hash.BytesToHash256(h[:]), nil
}

func rlpSignedHash(tx *types.Transaction, chainID uint32, sig []byte) (hash.Hash256, error) {
	signedTx, err := reconstructSignedRlpTxFromSig(tx, chainID, sig)
	if err != nil {
		return hash.ZeroHash256, err
	}
	h := sha3.NewLegacyKeccak256()
	rlp.Encode(h, signedTx)
	return hash.BytesToHash256(h.Sum(nil)), nil
}

func reconstructSignedRlpTxFromSig(rawTx *types.Transaction, chainID uint32, sig []byte) (*types.Transaction, error) {
	if len(sig) != 65 {
		return nil, errors.Errorf("invalid signature length = %d, expecting 65", len(sig))
	}
	sc := make([]byte, 65)
	copy(sc, sig)
	if sc[64] >= 27 {
		sc[64] -= 27
	}

	signedTx, err := rawTx.WithSignature(types.NewEIP155Signer(big.NewInt(int64(chainID))), sc)
	if err != nil {
		return nil, err
	}
	return signedTx, nil
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
	v, r, s := tx.RawSignatureValues()
	recID := uint32(v.Int64()) - 2*chainID - 8
	sig = make([]byte, 64, 65)
	rSize := len(r.Bytes())
	copy(sig[32-rSize:32], r.Bytes())
	sSize := len(s.Bytes())
	copy(sig[64-sSize:], s.Bytes())
	sig = append(sig, byte(recID))

	// recover public key
	rawHash := types.NewEIP155Signer(big.NewInt(int64(chainID))).Hash(tx)
	pubkey, err = crypto.RecoverPubkey(rawHash[:], sig)
	return
}
