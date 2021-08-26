package action

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/config"
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

func rlpRawHash(tx rlpTransaction, chainID uint32) (hash.Hash256, error) {
	rawTx, err := generateRlpTx(tx)
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

func generateRlpTx(act rlpTransaction) (*types.Transaction, error) {
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

	rawTx, err := generateRlpTx(tx)
	if err != nil {
		return nil, err
	}
	signedTx, err := rawTx.WithSignature(types.NewEIP155Signer(big.NewInt(int64(chainID))), sc)
	if err != nil {
		return nil, err
	}
	return signedTx, nil
}

func GetRLPSig(act Action, pvk crypto.PrivateKey) []byte {
	// register the extern chain ID
	fmt.Printf("CHAINID: %d\n", int64(config.EVMNetworkID()))
	// convert staking action into native tx
	rlpAct, err := actionToRLP(act)
	if err != nil {
		return []byte{}
	}
	rawTx, err := generateRlpTx(rlpAct)
	if err != nil {
		return []byte{}
	}
	fmt.Printf("data length = %d\n", len(rlpAct.Payload()))

	// generate signature from r, s in native signed tx
	ecdsaPvk, ok := pvk.EcdsaPrivateKey().(*ecdsa.PrivateKey)
	if !ok {
		return []byte{}
	}
	signer := types.NewEIP155Signer(big.NewInt(int64(config.EVMNetworkID())))
	signedNativeTx, err := types.SignTx(rawTx, signer, ecdsaPvk)
	if err != nil {
		return []byte{}
	}
	w, r, s := signedNativeTx.RawSignatureValues()
	recID := uint32(w.Int64()) - 2*config.EVMNetworkID() - 8
	sig := make([]byte, 64, 65)
	rSize := len(r.Bytes())
	copy(sig[32-rSize:32], r.Bytes())
	sSize := len(s.Bytes())
	copy(sig[64-sSize:], s.Bytes())
	sig = append(sig, byte(recID))
	fmt.Printf("signature = %x\n", sig)
	fmt.Printf("w = %d, networkId = %d, recID = %d\n", w.Int64(), config.EVMNetworkID(), recID)
	return sig
}
