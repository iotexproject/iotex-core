// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package filedao

import (
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"

	"github.com/iotexproject/go-pkgs/hash"

	"github.com/iotexproject/iotex-core/blockchain/filedao/headerpb"
	"github.com/iotexproject/iotex-core/db"
)

type (
	// FileHeader is header of chain db file
	FileHeader struct {
		Version        string
		Compressor     string
		BlockStoreSize uint64
		Start          uint64
	}

	// FileTip is tip info of chain
	FileTip struct {
		Height uint64
		Hash   hash.Hash256
	}
)

// ReadHeader reads header from KVStore
func ReadHeader(kv db.KVStore, ns string, key []byte) (*FileHeader, error) {
	value, err := kv.Get(ns, key)
	if err != nil {
		return nil, errors.Wrap(err, "file header not exist")
	}
	return DeserializeFileHeader(value)
}

// WriteHeader writes header to KVStore
func WriteHeader(kv db.KVStore, ns string, key []byte, header *FileHeader) error {
	ser, err := header.Serialize()
	if err != nil {
		return err
	}
	return kv.Put(ns, key, ser)
}

// Serialize serializes FileHeader to byte-stream
func (h *FileHeader) Serialize() ([]byte, error) {
	return proto.Marshal(h.toProto())
}

func (h *FileHeader) toProto() *headerpb.FileHeader {
	return &headerpb.FileHeader{
		Version:        h.Version,
		Compressor:     h.Compressor,
		BlockStoreSize: h.BlockStoreSize,
		Start:          h.Start,
	}
}

func fromProtoFileHeader(pb *headerpb.FileHeader) *FileHeader {
	return &FileHeader{
		Version:        pb.Version,
		Compressor:     pb.Compressor,
		BlockStoreSize: pb.BlockStoreSize,
		Start:          pb.Start,
	}
}

// DeserializeFileHeader deserializes byte-stream to FileHeader
func DeserializeFileHeader(buf []byte) (*FileHeader, error) {
	pbHeader := &headerpb.FileHeader{}
	if err := proto.Unmarshal(buf, pbHeader); err != nil {
		return nil, err
	}
	return fromProtoFileHeader(pbHeader), nil
}

// ReadTip reads tip from KVStore
func ReadTip(kv db.KVStore, ns string, key []byte) (*FileTip, error) {
	value, err := kv.Get(ns, key)
	if err != nil {
		return nil, errors.Wrap(err, "file header not exist")
	}
	return DeserializeFileTip(value)
}

// WriteTip writes tip to KVStore
func WriteTip(kv db.KVStore, ns string, key []byte, tip *FileTip) error {
	ser, err := tip.Serialize()
	if err != nil {
		return err
	}
	return kv.Put(ns, key, ser)
}

// Serialize serializes FileTip to byte-stream
func (t *FileTip) Serialize() ([]byte, error) {
	return proto.Marshal(t.toProto())
}

func (t *FileTip) toProto() *headerpb.FileTip {
	return &headerpb.FileTip{
		Height: t.Height,
		Hash:   t.Hash[:],
	}
}

func fromProtoFileTip(pb *headerpb.FileTip) *FileTip {
	return &FileTip{
		Height: pb.Height,
		Hash:   hash.BytesToHash256(pb.Hash),
	}
}

// DeserializeFileTip deserializes byte-stream to FileTip
func DeserializeFileTip(buf []byte) (*FileTip, error) {
	pbTip := &headerpb.FileTip{}
	if err := proto.Unmarshal(buf, pbTip); err != nil {
		return nil, err
	}
	return fromProtoFileTip(pbTip), nil
}
