package erigonstore

import (
	erigonComm "github.com/erigontech/erigon-lib/common"
	"github.com/erigontech/erigon/core/types/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/gogo/protobuf/proto"
	"github.com/holiman/uint256"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action/protocol/account/accountpb"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/systemcontracts"
)

type accountStorage struct {
	backend  *contractBackend
	contract systemcontracts.StorageContract
}

func newAccountStorage(addr common.Address, backend *contractBackend) (*accountStorage, error) {
	contract, err := systemcontracts.NewGenericStorageContract(
		addr,
		backend,
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create account storage contract")
	}
	return &accountStorage{
		backend:  backend,
		contract: contract,
	}, nil
}

func (as *accountStorage) Delete([]byte) error {
	return errors.New("not implemented")
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
	// load other fields from the account storage contract
	value, err := as.contract.Get(addr.Bytes())
	if err != nil {
		return errors.Wrapf(err, "failed to get account data for address %x", addr.Bytes())
	}
	if !value.KeyExists {
		return errors.Errorf("account info not found for address %x", addr.Bytes())
	}
	pbAcc := &accountpb.Account{}
	if err := proto.Unmarshal(value.Value.PrimaryData, pbAcc); err != nil {
		return errors.Wrapf(err, "failed to unmarshal account data for address %x", addr.Bytes())
	}

	balance := as.backend.intraBlockState.GetBalance(addr)
	nonce := as.backend.intraBlockState.GetNonce(addr)
	pbAcc.Balance = balance.String()
	switch pbAcc.Type {
	case accountpb.AccountType_ZERO_NONCE:
		pbAcc.Nonce = nonce
	case accountpb.AccountType_DEFAULT:
		pbAcc.Nonce = nonce - 1
	default:
		return errors.Errorf("unknown account type %v for address %x", pbAcc.Type, addr.Bytes())
	}

	if ch := as.backend.intraBlockState.GetCodeHash(addr); !accounts.IsEmptyCodeHash(ch) {
		pbAcc.CodeHash = ch.Bytes()
	}
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
	pbAcc.CodeHash = nil
	data, err := proto.Marshal(pbAcc)
	if err != nil {
		return errors.Wrapf(err, "failed to marshal account %x", addr.Bytes())
	}

	return as.contract.Put(key, systemcontracts.GenericValue{PrimaryData: data})
}
