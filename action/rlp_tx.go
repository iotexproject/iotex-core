// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"encoding/hex"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
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

// NewEthSigner returns the proper signer for Eth-compatible tx
func NewEthSigner(txType iotextypes.Encoding, chainID uint32) (types.Signer, error) {
	switch txType {
	case iotextypes.Encoding_IOTEX_PROTOBUF, iotextypes.Encoding_ETHEREUM_UNPROTECTED:
		// native tx use same signature format as that of Homestead (for pre-EIP155 unprotected tx)
		return types.HomesteadSigner{}, nil
	case iotextypes.Encoding_ETHEREUM_EIP155:
		return types.NewCancunSigner(big.NewInt(int64(chainID))), nil
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
		encoding = iotextypes.Encoding_ETHEREUM_EIP155
		signer   = types.NewCancunSigner(tx.ChainId()) // by default assume latest signer
		V, R, S  = tx.RawSignatureValues()
	)
	// extract correct V value
	switch tx.Type() {
	case types.LegacyTxType:
		if tx.Protected() {
			chainIDMul := tx.ChainId()
			V = new(big.Int).Sub(V, new(big.Int).Lsh(chainIDMul, 1))
			V.Sub(V, big.NewInt(8))
		} else {
			// tx has pre-EIP155 signature
			encoding = iotextypes.Encoding_ETHEREUM_UNPROTECTED
			signer = types.HomesteadSigner{}
		}
	case types.AccessListTxType, types.DynamicFeeTxType, types.BlobTxType:
		// AL txs are defined to use 0 and 1 as their recovery id,
		// DynamicFee txs are defined to use 0 and 1 as their recovery id,
		// Blob txs are defined to use 0 and 1 as their recovery id,
		// add 27 to become equivalent to unprotected Homestead signatures.
		V = new(big.Int).Add(V, big.NewInt(27))
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

func StakingRewardingTxToEnvelope(chainID uint32, tx *types.Transaction) (Envelope, error) {
	if tx.To() == nil {
		return nil, nil
	}
	var (
		ioAddr, _  = address.FromBytes(tx.To().Bytes())
		to         = ioAddr.String()
		elpBuilder = (&EnvelopeBuilder{}).SetChainID(chainID)
	)
	if to == address.StakingProtocolAddr {
		return elpBuilder.BuildStakingAction(tx)
	}
	if to == address.RewardingProtocol {
		return elpBuilder.BuildRewardingAction(tx)
	}
	return nil, nil
}

func EthRawToContainer(chainID uint32, rawString string) (*iotextypes.ActionCore, error) {
	if strings.HasPrefix(rawString, "0x") || strings.HasPrefix(rawString, "0X") {
		rawString = rawString[2:]
	}
	rawBytes, err := hex.DecodeString(rawString)
	if err != nil {
		return nil, err
	}
	return &iotextypes.ActionCore{
		ChainID: chainID,
		Action: &iotextypes.ActionCore_TxContainer{
			TxContainer: &iotextypes.TxContainer{
				Raw: rawBytes,
			},
		},
	}, nil
}
