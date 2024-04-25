// Copyright (c) 2023 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package db

import (
	"fmt"

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

// keyMeta is the metadata for key's index
type keyMeta struct {
	lastWrite     []byte
	firstVersion  uint64
	lastVersion   uint64
	deleteVersion []uint64
	dawVersion    []uint64 // delete-after-write version
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
		DawVersion:    k.dawVersion,
	}
}

func fromProtoKM(pb *versionpb.KeyMeta) *keyMeta {
	return &keyMeta{
		lastWrite:     pb.LastWrite,
		firstVersion:  pb.FirstVersion,
		lastVersion:   pb.LastVersion,
		deleteVersion: pb.DeleteVersion,
		dawVersion:    pb.DawVersion,
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

func (km *keyMeta) checkRead(version uint64) (bool, error) {
	if km == nil || version < km.firstVersion {
		return false, ErrNotExist
	}
	if version < km.lastVersion {
		return false, nil
	}
	return km.hitLastWrite(km.lastVersion, version)
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
	lastDelete := km.lastDelete()
	if version < km.lastVersion || version < lastDelete {
		// writing to an earlier version complicates things, for now it is not allowed
		return km, true
	}
	if version == lastDelete {
		// if the last delete is a delete-after-write, clear it
		km.clearLastDAW(version)
	}
	km.lastWrite = value
	km.lastVersion = version
	return km, false
}

func (km *keyMeta) updateDelete(version uint64) error {
	if version < km.lastVersion || version < km.lastDelete() {
		// not allowed to delete an earlier version
		return ErrInvalid
	}
	km.deleteVersion = append(km.deleteVersion, version)
	if version == km.lastVersion {
		// mark it as delete-after-write
		km.dawVersion = append(km.dawVersion, version)
	}
	return nil
}

func (km *keyMeta) lastDelete() uint64 {
	if numDelete := len(km.deleteVersion); numDelete > 0 {
		return km.deleteVersion[numDelete-1]
	}
	return 0
}

func (km *keyMeta) isDeleteAfterWrite(v uint64) bool {
	for _, version := range km.dawVersion {
		if v == version {
			return true
		}
	}
	return false
}

func (km *keyMeta) clearLastDAW(v uint64) {
	size := len(km.dawVersion)
	if size > 0 && km.dawVersion[size-1] == v {
		km.dawVersion = km.dawVersion[:size-1]
	}
}

func (km *keyMeta) hitLastWrite(write, read uint64) (bool, error) {
	if write > read {
		panic(fmt.Sprintf("last write %d > attempted read %d", write, read))
	}
	var (
		nextDelete uint64
		hasDelete  bool
	)
	for _, v := range km.deleteVersion {
		if v >= write {
			nextDelete = v
			hasDelete = (write <= nextDelete && nextDelete <= read)
			break
		}
	}
	if !hasDelete {
		return true, nil
	}
	if write < nextDelete {
		// there's a delete after last write
		return false, ErrDeleted
	}
	if km.isDeleteAfterWrite(write) {
		// this is a delete-after-write on the same version
		return false, ErrDeleted
	}
	return true, nil
}
