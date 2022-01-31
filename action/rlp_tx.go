package action

import (
	"encoding/hex"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/go-pkgs/util"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"golang.org/x/crypto/sha3"
)

type rlpTransaction interface {
	Nonce() uint64
	GasPrice() *big.Int
	GasLimit() uint64
	Recipient() string
	Amount() *big.Int
	Payload() []byte
}

type actionType uint32

const (
	unsupportedTxType actionType = 0
	legacyTxType      actionType = 1
	ledgerTxType      actionType = 2

	ledgerTxByte byte = 0x71 // TODO: to be dertermined
)

func rlpRawHash(tx rlpTransaction, chainID uint32) (hash.Hash256, error) {
	rawTx, err := rlpToEthTx(tx)
	if err != nil {
		return hash.ZeroHash256, err
	}
	h := types.NewEIP155Signer(big.NewInt(int64(chainID))).Hash(rawTx)
	return hash.BytesToHash256(h[:]), nil
}

func rlpSignedHash(tx rlpTransaction, chainID uint32, sig []byte) (hash.Hash256, error) {
	signedTx, err := reconstructSignedRlpTxFromSig(tx, chainID, sig)
	if err != nil {
		return hash.ZeroHash256, err
	}
	h := sha3.NewLegacyKeccak256()
	rlp.Encode(h, signedTx)
	return hash.BytesToHash256(h.Sum(nil)), nil
}

func rlpToEthTx(act rlpTransaction) (*types.Transaction, error) {
	if act == nil {
		return nil, errors.New("nil action to generate RLP tx")
	}

	// generate raw tx
	if to := act.Recipient(); to != EmptyAddress {
		addr, err := address.FromString(to)
		if err != nil {
			return nil, errors.Wrapf(err, "invalid recipient address %s", to)
		}
		ethAddr := common.BytesToAddress(addr.Bytes())
		return types.NewTransaction(act.Nonce(), ethAddr, act.Amount(), act.GasLimit(), act.GasPrice(), act.Payload()), nil
	}
	return types.NewContractCreation(act.Nonce(), act.Amount(), act.GasLimit(), act.GasPrice(), act.Payload()), nil
}

func reconstructSignedRlpTxFromSig(tx rlpTransaction, chainID uint32, sig []byte) (*types.Transaction, error) {
	if len(sig) != 65 {
		return nil, errors.Errorf("invalid signature length = %d, expecting 65", len(sig))
	}
	sc := make([]byte, 65)
	copy(sc, sig)
	if sc[64] >= 27 {
		sc[64] -= 27
	}

	rawTx, err := rlpToEthTx(tx)
	if err != nil {
		return nil, err
	}
	signedTx, err := rawTx.WithSignature(types.NewEIP155Signer(big.NewInt(int64(chainID))), sc)
	if err != nil {
		return nil, err
	}
	return signedTx, nil
}

// DecodeRawTx decodes raw data string into eth tx
func DecodeRawTx(rawData string, chainID uint32) (*types.Transaction, []byte, bool, error) {
	//remove Hex prefix and decode string to byte
	dataInBytes, err := hex.DecodeString(util.Remove0xPrefix(rawData))
	if err != nil {
		return nil, nil, false, err
	}

	// handle special action if transactionType(eip-2718) is set
	actType, dataInBytes := handleSpecialActionType(dataInBytes)
	if actType == unsupportedTxType {
		return nil, nil, false, errors.New("unsupported action type")
	}

	// decode raw data into eth tx
	tx := &types.Transaction{}
	if rlp.DecodeBytes(dataInBytes, tx) != nil {
		return nil, nil, false, err
	}

	// extract signature
	v, r, s := tx.RawSignatureValues()
	recID := uint32(v.Int64()) - 2*chainID - 8
	sig := make([]byte, 64, 65)
	rSize := len(r.Bytes())
	copy(sig[32-rSize:32], r.Bytes())
	sSize := len(s.Bytes())
	copy(sig[64-sSize:], s.Bytes())
	sig = append(sig, byte(recID))

	return tx, sig, actType == legacyTxType, nil
}

func handleSpecialActionType(data []byte) (actionType, []byte) {
	if len(data) == 0 {
		return legacyTxType, data
	}
	if data[0] > 0x7f {
		return legacyTxType, data
	}
	switch data[0] {
	case ledgerTxByte:
		return ledgerTxType, data[1:]
	default:
		return unsupportedTxType, data
	}
}
