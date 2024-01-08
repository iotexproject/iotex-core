package action

import (
	"encoding/hex"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
)

func rlpRawHash(rawTx *types.Transaction, signer types.Signer) (hash.Hash256, error) {
	h := signer.Hash(rawTx)
	return hash.BytesToHash256(h[:]), nil
}

func rlpSignedHash(tx *types.Transaction, signer types.Signer, sig []byte) (hash.Hash256, error) {
	signedTx, err := RawTxToSignedTx(tx, signer, sig)
	if err != nil {
		return hash.ZeroHash256, err
	}
	h := signedTx.Hash()
	return hash.BytesToHash256(h[:]), nil
}

// RawTxToSignedTx converts the raw tx to corresponding signed tx
func RawTxToSignedTx(rawTx *types.Transaction, signer types.Signer, sig []byte) (*types.Transaction, error) {
	if len(sig) != 65 {
		return nil, errors.Errorf("invalid signature length = %d, expecting 65", len(sig))
	}
	sc := make([]byte, 65)
	copy(sc, sig)
	if sc[64] >= 27 {
		sc[64] -= 27
	}

	signedTx, err := rawTx.WithSignature(signer, sc)
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
	sig = make([]byte, 65)
	rSize := len(r.Bytes())
	copy(sig[32-rSize:32], r.Bytes())
	sSize := len(s.Bytes())
	copy(sig[64-sSize:], s.Bytes())
	sig[64] = byte(recID)

	// recover public key
	rawHash := types.NewEIP155Signer(big.NewInt(int64(chainID))).Hash(tx)
	pubkey, err = crypto.RecoverPubkey(rawHash[:], sig)
	return
}

// NewEthSigner returns the proper signer for Eth-compatible tx
func NewEthSigner(txType iotextypes.Encoding, chainID uint32) (types.Signer, error) {
	switch txType {
	case iotextypes.Encoding_IOTEX_PROTOBUF, iotextypes.Encoding_ETHEREUM_UNPROTECTED:
		// native tx use same signature format as that of Homestead (for pre-EIP155 unprotected tx)
		return types.HomesteadSigner{}, nil
	case iotextypes.Encoding_ETHEREUM_EIP155, iotextypes.Encoding_ETHEREUM_ACCESSLIST:
		return types.NewEIP2930Signer(big.NewInt(int64(chainID))), nil
	default:
		return nil, ErrInvalidAct
	}
}

// DecodeEtherTx decodes raw data string into eth tx
func DecodeEtherTx(rawData string) (*types.Transaction, error) {
	//remove Hex prefix and decode string to byte
	if strings.HasPrefix(rawData, "0x") || strings.HasPrefix(rawData, "0X") {
		rawData = rawData[2:]
	}
	rawTxBytes, err := hex.DecodeString(rawData)
	if err != nil {
		return nil, err
	}

	// decode raw data into eth tx
	tx := types.Transaction{}
	if err = tx.UnmarshalBinary(rawTxBytes); err != nil {
		return nil, err
	}
	return &tx, nil
}

// ExtractTypeSigPubkey extracts tx type, signature, and pubkey
func ExtractTypeSigPubkey(tx *types.Transaction) (iotextypes.Encoding, []byte, crypto.PublicKey, error) {
	var (
		encoding iotextypes.Encoding
		signer   = types.NewEIP2930Signer(tx.ChainId()) // by default assume latest signer
		V, R, S  = tx.RawSignatureValues()
	)
	// extract correct V value
	switch tx.Type() {
	case types.LegacyTxType:
		if tx.Protected() {
			chainIDMul := tx.ChainId()
			V = new(big.Int).Sub(V, new(big.Int).Lsh(chainIDMul, 1))
			V.Sub(V, big.NewInt(8))
			encoding = iotextypes.Encoding_ETHEREUM_EIP155
		} else {
			// tx has pre-EIP155 signature
			encoding = iotextypes.Encoding_ETHEREUM_UNPROTECTED
			signer = types.HomesteadSigner{}
		}
	case types.AccessListTxType:
		// AL txs are defined to use 0 and 1 as their recovery
		// id, add 27 to become equivalent to unprotected Homestead signatures.
		V = new(big.Int).Add(V, big.NewInt(27))
		encoding = iotextypes.Encoding_ETHEREUM_ACCESSLIST
	default:
		return encoding, nil, nil, ErrNotSupported
	}

	// construct signature
	if V.BitLen() > 8 {
		return encoding, nil, nil, ErrNotSupported
	}

	var (
		r, s   = R.Bytes(), S.Bytes()
		sig    = make([]byte, 65)
		pubkey crypto.PublicKey
		err    error
	)
	copy(sig[32-len(r):32], r)
	copy(sig[64-len(s):64], s)
	sig[64] = byte(V.Uint64())

	// recover public key
	rawHash := signer.Hash(tx)
	pubkey, err = crypto.RecoverPubkey(rawHash[:], sig)
	return encoding, sig, pubkey, err
}
