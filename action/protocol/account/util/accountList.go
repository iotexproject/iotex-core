package accountutil

import (
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action/protocol/account/accountpb"
)

// AccountList indicates the list of accounts
type AccountList []string

// Serialize serializes a list of accounts to bytes
func (al *AccountList) Serialize() ([]byte, error) {
	return proto.Marshal(al.proto())
}

// Deserialize deserializes bytes to list of accounts
func (al *AccountList) Deserialize(buf []byte) error {
	accountListPb := &accountpb.AccountList{}
	if err := proto.Unmarshal(buf, accountListPb); err != nil {
		return errors.Wrap(err, "failed to unmarshal account list")
	}
	*al = accountListPb.Address
	return nil
}

func (al *AccountList) proto() *accountpb.AccountList {
	address := make([]string, 0, len(*al))
	for _, addr := range *al {
		address = append(address, addr)
	}
	return &accountpb.AccountList{Address: address}
}
