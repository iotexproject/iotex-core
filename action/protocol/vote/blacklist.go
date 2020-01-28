// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package vote

import (
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action/protocol/vote/blacklistpb"
)

//Blacklist defines a map where key is candidate's name and value is the counter which counts the unproductivity during kick-out epoch.
type Blacklist map[string]int32

// Serialize serializes map of blacklist to bytes
func (bl *Blacklist) Serialize() ([]byte, error) {
	return proto.Marshal(bl.Proto())
}

// Proto converts the blacklist to a protobuf message
func (bl *Blacklist) Proto() *blacklistpb.BlackList {
	delegates := make([]*blacklistpb.BlackListInfo, 0, len(*bl))
	for name, count := range *bl {
		delegatepb := &blacklistpb.BlackListInfo{
			Name:  name,
			Count: count,
		}
		delegates = append(delegates, delegatepb)
	}
	return &blacklistpb.BlackList{
		BlackListInfos: delegates,
	}
}

// Deserialize deserializes bytes to delegate blacklist
func (bl *Blacklist) Deserialize(buf []byte) error {
	blackList := &blacklistpb.BlackList{}
	if err := proto.Unmarshal(buf, blackList); err != nil {
		return errors.Wrap(err, "failed to unmarshal blacklist")
	}
	return bl.LoadProto(blackList)
}

// LoadProto loads blacklist from proto
func (bl *Blacklist) LoadProto(blackList *blacklistpb.BlackList) error {
	blackListMap := make(map[string]int32, 0)
	delegates := blackList.BlackListInfos
	for _, delegate := range delegates {
		blackListMap[delegate.Name] = delegate.Count
	}
	*bl = blackListMap

	return nil
}
