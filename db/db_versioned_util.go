// Copyright (c) 2023 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package db

import (
	"github.com/iotexproject/go-pkgs/byteutil"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/db/versionpb"
)

// versionedNamespace is the metadata for versioned namespace
type versionedNamespace struct {
	keyLen uint32
}

// serialize to bytes
func (vn *versionedNamespace) serialize() []byte {
	return byteutil.Must(proto.Marshal(vn.toProto()))
}

func (vn *versionedNamespace) toProto() *versionpb.VersionedNamespace {
	return &versionpb.VersionedNamespace{
		KeyLen: vn.keyLen,
	}
}

func fromProtoVN(pb *versionpb.VersionedNamespace) *versionedNamespace {
	return &versionedNamespace{
		keyLen: pb.KeyLen,
	}
}

// deserializeVersionedNamespace deserializes byte-stream to VersionedNamespace
func deserializeVersionedNamespace(buf []byte) (*versionedNamespace, error) {
	var vn versionpb.VersionedNamespace
	if err := proto.Unmarshal(buf, &vn); err != nil {
		return nil, err
	}
	return fromProtoVN(&vn), nil
}
