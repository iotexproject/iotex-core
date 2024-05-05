// Copyright (c) 2023 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package db

import (
	"github.com/iotexproject/go-pkgs/byteutil"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/db/versionpb"
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

// keyMeta is the metadata for key's index
type keyMeta struct {
	lastWrite     []byte
	firstVersion  uint64
	lastVersion   uint64
	deleteVersion uint64
}

// serialize to bytes
func (k *keyMeta) serialize() []byte {
	return byteutil.Must(proto.Marshal(k.toProto()))
}

func (k *keyMeta) toProto() *versionpb.KeyMeta {
	return &versionpb.KeyMeta{
		LastWrite:     k.lastWrite,
		FirstVersion:  k.firstVersion,
		LastVersion:   k.lastVersion,
		DeleteVersion: k.deleteVersion,
	}
}

func fromProtoKM(pb *versionpb.KeyMeta) *keyMeta {
	return &keyMeta{
		lastWrite:     pb.LastWrite,
		firstVersion:  pb.FirstVersion,
		lastVersion:   pb.LastVersion,
		deleteVersion: pb.DeleteVersion,
	}
}

// deserializeKeyMeta deserializes byte-stream to key meta
func deserializeKeyMeta(buf []byte) (*keyMeta, error) {
	var km versionpb.KeyMeta
	if err := proto.Unmarshal(buf, &km); err != nil {
		return nil, err
	}
	return fromProtoKM(&km), nil
}

func (km *keyMeta) updateRead(version uint64) (bool, error) {
	if km == nil || version < km.firstVersion {
		return false, ErrNotExist
	}
	if km.deleteVersion != 0 && version >= km.deleteVersion {
		return false, ErrDeleted
	}
	return (version >= km.lastVersion), nil
}

func (km *keyMeta) updateWrite(version uint64, value []byte) (*keyMeta, bool) {
	if km == nil {
		// key not yet written
		return &keyMeta{
			lastWrite:    value,
			firstVersion: version,
			lastVersion:  version,
		}, false
	}
	if version < km.lastVersion || version < km.deleteVersion {
		// writing to an earlier version complicates things, for now it is not allowed
		return km, true
	}
	km.lastWrite = value
	km.lastVersion = version
	km.deleteVersion = 0
	return km, false
}

func (km *keyMeta) updatePurge(version uint64) (uint64, bool) {
	if km.firstVersion == km.lastVersion || version < km.firstVersion {
		// key has been written only once (the last version will always be kept)
		// or version is lower than the version when the key is first written
		return version, true
	}
	if version >= km.lastVersion {
		// delete all versions except the last version
		version = km.lastVersion - 1
	}
	return version, false
}
