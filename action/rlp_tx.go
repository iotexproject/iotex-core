package action

import (
	"crypto/ecdsa"
	"encoding/hex"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/go-pkgs/util"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"golang.org/x/crypto/sha3"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

type rlpTransaction interface {
	Nonce() uint64
	GasPrice() *big.Int
	GasLimit() uint64
	Recipient() string
	Amount() *big.Int
	Payload() []byte
}

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
func DecodeRawTx(rawData string, chainID uint32) (*types.Transaction, bool, error) {
	//remove Hex prefix and decode string to byte
	dataInString, err := hex.DecodeString(util.Remove0xPrefix(rawData))
	if err != nil {
		return nil, false, err
	}

	// decode raw data into rlp tx
	unwrapper := &transactionUnwrapper{}
	if err = rlp.DecodeBytes(dataInString, unwrapper); err != nil {
		return nil, false, err
	}
	return unwrapper.tx, unwrapper.isEthEncoding, nil
}

// EncodeRawTx encodes action into the data string of eth tx
func EncodeRawTx(act Action, pvk crypto.PrivateKey, chainID uint32) (string, error) {
	rlpAct, err := actionToRLP(act)
	if err != nil {
		return "", err
	}
	rawTx, err := rlpToEthTx(rlpAct)
	if err != nil {
		return "", err
	}
	ecdsaPvk, ok := pvk.EcdsaPrivateKey().(*ecdsa.PrivateKey)
	if !ok {
		return "", errors.New("private key is invalid")
	}

	signer := types.NewEIP155Signer(big.NewInt(int64(chainID)))
	signedTx, err := types.SignTx(rawTx, signer, ecdsaPvk)
	if err != nil {
		return "", err
	}
	encodedTx, err := rlp.EncodeToBytes(signedTx)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(encodedTx[:]), nil
}

// ExtractSignatureAndPubkey calculates signature and pubkey from tx
func ExtractSignatureAndPubkey(tx *types.Transaction, pbAct *iotextypes.ActionCore, chainID uint32, isEthEncoding bool) ([]byte, crypto.PublicKey, error) {
	// extract signature and hash
	var (
		sig     []byte
		rawHash []byte
		err     error
	)
	sig, err = getSignatureFromRLPTX(tx, chainID)
	if err != nil {
		return nil, nil, err
	}
	if isEthEncoding {
		h := types.NewEIP155Signer(big.NewInt(int64(chainID))).Hash(tx)
		rawHash = h[:]
	} else {
		h := hash.Hash256b(byteutil.Must(proto.Marshal(pbAct)))
		rawHash = h[:]
	}

	// recover public key
	pubkey, err := crypto.RecoverPubkey(rawHash, sig)
	if err != nil {
		return nil, nil, err
	}
	return sig, pubkey, nil
}

func getSignatureFromRLPTX(tx *types.Transaction, chainID uint32) ([]byte, error) {
	if tx == nil {
		return nil, errors.New("pointer is nil")
	}
	v, r, s := tx.RawSignatureValues()

	recID := uint32(v.Int64()) - 2*chainID - 8
	sig := make([]byte, 64, 65)
	rSize := len(r.Bytes())
	copy(sig[32-rSize:32], r.Bytes())
	sSize := len(s.Bytes())
	copy(sig[64-sSize:], s.Bytes())
	sig = append(sig, byte(recID))
	return sig, nil
}

type (
	// TransactionUnwrapper is a unwrapper for the Ethereum transaction.
	transactionUnwrapper struct {
		tx            *types.Transaction
		isEthEncoding bool
	}

	legacyTxWithEncodingType struct {
		// data fields of legacy transaction.
		Nonce    uint64
		GasPrice *big.Int
		Gas      uint64
		To       *common.Address `rlp:"nil"`
		Value    *big.Int
		Data     []byte
		V, R, S  *big.Int
		// extra field to identify tx signed from ledgers which use native signing method
		EncodingType []byte `rlp:"optional"`
	}
)

// DecodeRLP implements eth.rlp.Decoder
func (unwrapper *transactionUnwrapper) DecodeRLP(s *rlp.Stream) error {
	kind, _, err := s.Kind()
	if err != nil {
		return err
	}
	switch kind {
	case rlp.List:
		outter := legacyTxWithEncodingType{}
		if err := s.Decode(&outter); err != nil {
			return err
		}
		unwrapper.tx = types.NewTx(outter.exportLegacyTx())
		unwrapper.isEthEncoding = outter.isEthEncoding()
		return nil
	default:
		return rlp.ErrExpectedList
	}
}

func (tx *legacyTxWithEncodingType) exportLegacyTx() *types.LegacyTx {
	return &types.LegacyTx{
		Nonce:    tx.Nonce,
		GasPrice: tx.GasPrice,
		Gas:      tx.Gas,
		To:       tx.To,
		Value:    tx.Value,
		Data:     tx.Data,
		V:        tx.V,
		R:        tx.R,
		S:        tx.S,
	}
}

func (tx *legacyTxWithEncodingType) isEthEncoding() bool {
	if len(tx.EncodingType) > 0 && hex.EncodeToString(tx.EncodingType) == "01" {
		return false
	}
	return true
}
