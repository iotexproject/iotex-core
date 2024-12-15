// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package factory

import (
	"context"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action/protocol/execution/evm"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/db/batch"
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
)

type daoRTF struct {
	dao db.KVStore
}

func newDaoRetrofitter(dao db.KVStore) *daoRTF {
	return &daoRTF{
		dao: dao,
	}
}

func (rtf *daoRTF) Start(ctx context.Context) error {
	return rtf.dao.Start(ctx)
}

func (rtf *daoRTF) Stop(ctx context.Context) error {
	return rtf.dao.Stop(ctx)
}

func (rtf *daoRTF) checkDBCompatibility() error {
	for _, ns := range VersionedNamespaces {
		if _, err := rtf.dao.Get(ns, []byte{0}); err == nil {
			return errors.Wrap(ErrIncompatibleDB, "non-archive using archive-mode DB")
		}
	}
	if _, err := rtf.dao.Get(VersionedMetadata, []byte(CurrentHeightKey)); err == nil {
		return errors.Wrap(ErrIncompatibleDB, "non-archive using archive-mode DB")
	}
	return nil
}

func (rtf *daoRTF) atHeight(uint64) db.KVStore {
	return rtf.dao
}

func (rtf *daoRTF) getHeight() (uint64, error) {
	height, err := rtf.dao.Get(AccountKVNamespace, []byte(CurrentHeightKey))
	if err != nil {
		return 0, errors.Wrap(err, "failed to get factory's height from underlying DB")
	}
	return byteutil.BytesToUint64(height), nil
}

func (rtf *daoRTF) putHeight(h uint64) error {
	return rtf.dao.Put(AccountKVNamespace, []byte(CurrentHeightKey), byteutil.Uint64ToBytes(h))
}

func (rtf *daoRTF) metadataNS() string {
	return AccountKVNamespace
}

func (rtf *daoRTF) flusherOptions(preEaster bool) []db.KVStoreFlusherOption {
	opts := []db.KVStoreFlusherOption{
		db.SerializeOption(func(wi *batch.WriteInfo) []byte {
			if preEaster {
				return wi.SerializeWithoutWriteType()
			}
			return wi.Serialize()
		}),
	}
	if !preEaster {
		return opts
	}
	return append(
		opts,
		db.SerializeFilterOption(func(wi *batch.WriteInfo) bool {
			return wi.Namespace() == evm.CodeKVNameSpace || wi.Namespace() == staking.CandsMapNS
		}),
	)
}
