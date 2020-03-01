// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package vote

import (
	"sort"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"

	updpb "github.com/iotexproject/iotex-core/action/protocol/vote/unproductivedelegatepb"
)

// UnproductiveDelegate defines unproductive delegates information within kickout period
type UnproductiveDelegate struct {
	delegatelist  [][]string
	kickoutPeriod uint64
	cacheSize     uint64
}

// NewUnproductiveDelegate creates new UnproductiveDelegate with kickoutperiod and cacheSize
func NewUnproductiveDelegate(kickoutPeriod uint64, cacheSize uint64) (*UnproductiveDelegate, error) {
	if kickoutPeriod > cacheSize {
		return nil, errors.New("cache size of unproductiveDelegate should be bigger than kickout period + 1")
	}
	return &UnproductiveDelegate{
		delegatelist:  make([][]string, cacheSize),
		kickoutPeriod: kickoutPeriod,
		cacheSize:     cacheSize,
	}, nil
}

// AddRecentUPD adds new epoch upd-list at the leftmost and shift existing lists to the right
func (upd *UnproductiveDelegate) AddRecentUPD(new []string) error {
	delegates := make([]string, len(new))
	copy(delegates, new)
	sort.Strings(delegates)
	upd.delegatelist = append([][]string{delegates}, upd.delegatelist[0:upd.kickoutPeriod-1]...)
	if len(upd.delegatelist) > int(upd.kickoutPeriod) {
		return errors.New("wrong length of UPD delegatelist")
	}
	return nil
}

// ReadOldestUPD returns the last upd-list
func (upd *UnproductiveDelegate) ReadOldestUPD() []string {
	return upd.delegatelist[upd.kickoutPeriod-1]
}

// Serialize serializes unproductvieDelegate struct to bytes
func (upd *UnproductiveDelegate) Serialize() ([]byte, error) {
	return proto.Marshal(upd.Proto())
}

// Proto converts the unproductvieDelegate struct to a protobuf message
func (upd *UnproductiveDelegate) Proto() *updpb.UnproductiveDelegate {
	delegatespb := make([]*updpb.Delegatelist, 0, len(upd.delegatelist))
	for _, elem := range upd.delegatelist {
		data := make([]string, len(elem))
		copy(data, elem)
		listpb := &updpb.Delegatelist{
			Delegates: data,
		}
		delegatespb = append(delegatespb, listpb)
	}
	return &updpb.UnproductiveDelegate{
		DelegateList:  delegatespb,
		KickoutPeriod: upd.kickoutPeriod,
		CacheSize:     upd.cacheSize,
	}
}

// Deserialize deserializes bytes to UnproductiveDelegate struct
func (upd *UnproductiveDelegate) Deserialize(buf []byte) error {
	unproductivedelegatePb := &updpb.UnproductiveDelegate{}
	if err := proto.Unmarshal(buf, unproductivedelegatePb); err != nil {
		return errors.Wrap(err, "failed to unmarshal blacklist")
	}
	return upd.LoadProto(unproductivedelegatePb)
}

// LoadProto converts protobuf message to unproductvieDelegate struct
func (upd *UnproductiveDelegate) LoadProto(updPb *updpb.UnproductiveDelegate) error {
	var delegates [][]string
	for _, delegatelistpb := range updPb.DelegateList {
		var delegateElem []string
		for _, str := range delegatelistpb.Delegates {
			delegateElem = append(delegateElem, str)
		}
		delegates = append(delegates, delegateElem)
	}
	upd.delegatelist = delegates
	upd.kickoutPeriod = updPb.KickoutPeriod
	upd.cacheSize = updPb.CacheSize

	return nil
}

// Equal compares with other upd struct and returns true if it's equal
func (upd *UnproductiveDelegate) Equal(upd2 *UnproductiveDelegate) bool {
	if upd.kickoutPeriod != upd2.kickoutPeriod {
		return false
	}
	if upd.cacheSize != upd2.cacheSize {
		return false
	}
	if len(upd.delegatelist) != len(upd2.delegatelist) {
		return false
	}
	for i, list := range upd.delegatelist {
		for j, str := range list {
			if str != upd2.delegatelist[i][j] {
				return false
			}
		}
	}
	return true
}

// DelegateList returns delegate list 2D array
func (upd *UnproductiveDelegate) DelegateList() [][]string {
	return upd.delegatelist
}
