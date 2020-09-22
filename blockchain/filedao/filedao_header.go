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
		End            uint64
		TopHash        hash.Hash256
	}
)

// ReadFileHeader reads header from KVStore
func ReadFileHeader(kv db.KVStore, ns string, key []byte) (*FileHeader, error) {
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
		End:            h.End,
		TopHash:        h.TopHash[:],
	}
}

func fromProto(pb *headerpb.FileHeader) *FileHeader {
	return &FileHeader{
		Version:        pb.Version,
		Compressor:     pb.Compressor,
		BlockStoreSize: pb.BlockStoreSize,
		Start:          pb.Start,
		End:            pb.End,
		TopHash:        hash.BytesToHash256(pb.TopHash),
	}
}

// DeserializeFileHeader deserializes byte-stream to FileHeader
func DeserializeFileHeader(buf []byte) (*FileHeader, error) {
	pbHeader := &headerpb.FileHeader{}
	if err := proto.Unmarshal(buf, pbHeader); err != nil {
		return nil, err
	}
	return fromProto(pbHeader), nil
}

func (h *FileHeader) createHeaderBytes(height uint64, hash hash.Hash256) ([]byte, error) {
	currHeight := h.End
	h.End = height
	h.TopHash = hash
	ser, err := h.Serialize()
	if err != nil {
		return nil, err
	}
	h.End = currHeight
	return ser, nil
}
