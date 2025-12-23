package erigonstore

import (
	"math/big"

	erigonComm "github.com/erigontech/erigon-lib/common"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/action/protocol/account/accountpb"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/systemcontracts"
)

type accountStorage struct {
	backend  *contractBackend
	contract systemcontracts.StorageContract
}

var (
	// Storage layout for deployed GenericStorage:
	// slot0: owner (Ownable)
	// slot1: keys_ (dynamic bytes array)
	// slot2: values_ (dynamic GenericValue array)
	// slot3: keyIndex_ (mapping bytes => uint256, storing index+1)
	accMappingSlot     = big.NewInt(3) // keyIndex_ mapping slot in GenericStorage
	accValuesArraySlot = big.NewInt(2) // values_ array slot in GenericStorage
	accValuesArrayBase = keccakSlot(accValuesArraySlot)
	accValueHeadSpan   = big.NewInt(3) // GenericValue has 3 dynamic bytes fields
	accMaxBytesPayload = uint64(16 << 20)
)

func newAccountStorage(addr common.Address, backend *contractBackend) (*accountStorage, error) {
	contract, err := systemcontracts.NewGenericStorageContract(
		addr,
		backend,
		common.Address(systemContractCreatorAddr),
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create account storage contract")
	}
	return &accountStorage{
		backend:  backend,
		contract: contract,
	}, nil
}

func (as *accountStorage) Delete(key []byte) error {
	exist, err := as.contract.Remove(key)
	if err != nil {
		return errors.Wrapf(err, "failed to remove account data for key %x", key)
	}
	if !exist {
		return errors.Wrapf(state.ErrStateNotExist, "key: %x", key)
	}
	return nil
}

func (as *accountStorage) Batch([][]byte) (state.Iterator, error) {
	return nil, errors.New("not implemented")
}

func (as *accountStorage) List() (state.Iterator, error) {
	return nil, errors.New("not implemented")
}

func (as *accountStorage) Load(key []byte, obj any) error {
	addr := erigonComm.BytesToAddress(key)
	acct, ok := obj.(*state.Account)
	if !ok {
		return errors.New("obj is not of type *state.Account")
	}
	if !as.backend.intraBlockState.Exist(addr) {
		return errors.Wrapf(state.ErrStateNotExist, "address: %x", addr.Bytes())
	}
	value, err := as.getBySlot(addr.Bytes())
	if err != nil {
		return errors.Wrapf(err, "failed to get account data for address %x", addr.Bytes())
	}
	if value == nil {
		return errors.Wrapf(state.ErrStateNotExist, "address: %x", addr.Bytes())
	}
	pbAcc := &accountpb.Account{}
	if err := proto.Unmarshal(value.PrimaryData, pbAcc); err != nil {
		return errors.Wrapf(err, "failed to unmarshal account data for address %x", addr.Bytes())
	}

	balance := as.backend.intraBlockState.GetBalance(addr)
	nonce := as.backend.intraBlockState.GetNonce(addr)
	pbAcc.Balance = balance.String()
	switch pbAcc.Type {
	case accountpb.AccountType_ZERO_NONCE:
		pbAcc.Nonce = nonce
	case accountpb.AccountType_DEFAULT:
		if nonce == 0 {
			pbAcc.Nonce = nonce
		} else {
			pbAcc.Nonce = nonce - 1
		}
	default:
		return errors.Errorf("unknown account type %v for address %x", pbAcc.Type, addr.Bytes())
	}
	// if ch := as.backend.intraBlockState.GetCodeHash(addr); !accounts.IsEmptyCodeHash(ch) {
	// 	pbAcc.CodeHash = ch.Bytes()
	// }
	pbAcc.CodeHash = as.backend.intraBlockState.GetCodeHash(addr).Bytes()
	acct.FromProto(pbAcc)
	return nil
}

func (as *accountStorage) Exists(key []byte) (bool, error) {
	return as.exists(key)
}

func (as *accountStorage) exists(key []byte) (bool, error) {
	addr := erigonComm.BytesToAddress(key)
	if !as.backend.intraBlockState.Exist(addr) {
		return false, nil
	}
	return true, nil
}

func (as *accountStorage) Store(key []byte, value any) error {
	if value == nil {
		return errors.New("value is nil")
	}
	acc, ok := value.(*state.Account)
	if !ok {
		return errors.New("value is not of type *state.Account")
	}
	addr := erigonComm.BytesToAddress(key)
	if !as.backend.intraBlockState.Exist(addr) {
		as.backend.intraBlockState.CreateAccount(addr, acc.IsContract())
	}
	as.backend.intraBlockState.SetBalance(addr, uint256.MustFromBig(acc.Balance))
	nonce := acc.PendingNonce()
	if as.backend.useZeroNonceForFreshAccount {
		nonce = acc.PendingNonceConsideringFreshAccount()
	}
	as.backend.intraBlockState.SetNonce(addr, nonce)
	// store other fields in the account storage contract
	pbAcc := acc.ToProto()
	pbAcc.Balance = ""
	pbAcc.Nonce = 0
	data, err := proto.Marshal(pbAcc)
	if err != nil {
		return errors.Wrapf(err, "failed to marshal account %x", addr.Bytes())
	}

	return as.contract.Put(key, systemcontracts.GenericValue{PrimaryData: data})
}

// getBySlot attempts to read data directly from storage slots for better performance.
// TODO: This function needs fixes - the storage slot calculation doesn't match actual GenericStorage layout.
// The mapping slot calculation for mapping(bytes => uint256) may need adjustment.
// For now, use contract.Get() in Load() which is functionally correct.
func (as *accountStorage) getBySlot(key []byte) (*systemcontracts.GenericValue, error) {
	idx, err := as.lookupIndex(key)
	if err != nil {
		return nil, err
	}
	if idx == nil {
		return nil, nil
	}

	valHead := valueHeadSlotAcc(idx)
	primary, err := as.readBytesAt(valHead)
	if err != nil {
		return nil, err
	}
	return &systemcontracts.GenericValue{PrimaryData: primary}, nil
}

// lookupIndex returns the zero-based index for the given key by probing both
// possible mapping hashing strategies used for bytes keys. Returns nil if not found.
func (as *accountStorage) lookupIndex(key []byte) (*big.Int, error) {
	// Strategy A: keccak256(key || slot)
	// Strategy B: keccak256(keccak256(key) || slot) (used by Solidity for bytes/string mapping keys)
	hashes := []common.Hash{
		keyIndexSlotAccConcat(key),
		keyIndexSlotAccHashed(key),
	}
	for _, idxSlot := range hashes {
		rawIdx, err := as.backend.StorageAt(as.contract.Address(), idxSlot)
		if err != nil {
			return nil, err
		}
		idx := new(big.Int).SetBytes(rawIdx)
		if idx.Sign() == 0 {
			continue
		}
		return idx.Sub(idx, big.NewInt(1)), nil
	}
	return nil, nil
}

func (as *accountStorage) readBytesAt(slotBI *big.Int) ([]byte, error) {
	word, err := as.backend.StorageAt(as.contract.Address(), common.BigToHash(slotBI))
	if err != nil {
		return nil, errors.Wrap(err, "read bytes head")
	}
	// Solidity bytes storage encoding:
	// - Short bytes (< 32 bytes): last byte = length*2 (even), data is left-aligned in slot
	// - Long bytes (>= 32 bytes): last byte = length*2+1 (odd), actual data starts at keccak256(slot)
	if len(word) == 32 && (word[31]&1) == 0 {
		// Inline short bytes
		l := int(word[31] >> 1)
		if l == 0 {
			return []byte{}, nil
		}
		if l > 31 {
			return nil, errors.Errorf("inline bytes length invalid: %d", l)
		}
		// Data is left-aligned in the slot
		return append([]byte{}, word[:l]...), nil
	}
	// Long bytes: slot contains length*2+1, actual data starts at keccak256(slot)
	length := new(big.Int).SetBytes(word)
	if length.Sign() == 0 {
		return []byte{}, nil
	}
	if length.BitLen() > 63 {
		return nil, errors.Errorf("bytes length too large: %s", length.String())
	}
	// For long bytes, Solidity stores length*2+1, so (length-1)/2 gives actual byte length
	byteLen := (length.Uint64() - 1) >> 1
	if byteLen > accMaxBytesPayload {
		return nil, errors.Errorf("bytes length exceeds guard: %d", byteLen)
	}
	dataBase := crypto.Keccak256Hash(padSlot(slotBI))
	words := (byteLen + 31) / 32
	buf := make([]byte, 0, byteLen+31)
	baseInt := new(big.Int).SetBytes(dataBase.Bytes())
	for i := uint64(0); i < words; i++ {
		wordSlot := new(big.Int).Add(baseInt, new(big.Int).SetUint64(i))
		w, err := as.backend.StorageAt(as.contract.Address(), common.BigToHash(wordSlot))
		if err != nil {
			return nil, errors.Wrapf(err, "read bytes word %d", i)
		}
		buf = append(buf, w...)
	}
	return buf[:byteLen], nil
}

// keyIndexSlotAccConcat computes keccak256(key || slot).
func keyIndexSlotAccConcat(key []byte) common.Hash {
	slotBytes := common.LeftPadBytes(accMappingSlot.Bytes(), 32)
	combined := append(key, slotBytes...)
	return crypto.Keccak256Hash(combined)
}

// keyIndexSlotAccHashed computes keccak256(keccak256(key) || slot), which is
// the strategy Solidity uses for mapping keys of dynamic types like bytes/string.
func keyIndexSlotAccHashed(key []byte) common.Hash {
	keyHash := crypto.Keccak256Hash(key)
	slotBytes := common.LeftPadBytes(accMappingSlot.Bytes(), 32)
	combined := append(keyHash.Bytes(), slotBytes...)
	return crypto.Keccak256Hash(combined)
}

func valueHeadSlotAcc(idx *big.Int) *big.Int {
	mul := new(big.Int).Mul(new(big.Int).Set(idx), accValueHeadSpan)
	return new(big.Int).Add(accValuesArrayBase, mul)
}

func keccakSlot(slot *big.Int) *big.Int {
	return new(big.Int).SetBytes(crypto.Keccak256(padSlot(slot)))
}

func padSlot(slot *big.Int) []byte {
	return common.LeftPadBytes(slot.Bytes(), 32)
}
