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

type daoRTFArchive struct {
	daoVersioned db.KvVersioned
	metaNS       string // namespace for metadata
}

func newDaoRetrofitterArchive(dao db.KvVersioned, mns string) *daoRTFArchive {
	return &daoRTFArchive{
		daoVersioned: dao,
		metaNS:       mns,
	}
}

func (rtf *daoRTFArchive) Start(ctx context.Context) error {
	return rtf.daoVersioned.Start(ctx)
}

func (rtf *daoRTFArchive) Stop(ctx context.Context) error {
	return rtf.daoVersioned.Stop(ctx)
}

func (rtf *daoRTFArchive) checkDBCompatibility() error {
	if _, err := rtf.daoVersioned.Base().Get(AccountKVNamespace, []byte(CurrentHeightKey)); err == nil {
		return errors.Wrap(ErrIncompatibleDB, "archive-mode using non-archive DB")
	}
	return nil
}

func (rtf *daoRTFArchive) atHeight(h uint64) db.KVStore {
	return rtf.daoVersioned.SetVersion(h)
}

func (rtf *daoRTFArchive) getHeight() (uint64, error) {
	height, err := rtf.daoVersioned.Base().Get(rtf.metaNS, []byte(CurrentHeightKey))
	if err != nil {
		return 0, errors.Wrap(err, "failed to get factory's height from underlying DB")
	}
	return byteutil.BytesToUint64(height), nil
}

func (rtf *daoRTFArchive) putHeight(h uint64) error {
	return rtf.daoVersioned.Base().Put(rtf.metaNS, []byte(CurrentHeightKey), byteutil.Uint64ToBytes(h))
}

func (rtf *daoRTFArchive) metadataNS() string {
	return rtf.metaNS
}

func (rtf *daoRTFArchive) flusherOptions(preEaster bool) []db.KVStoreFlusherOption {
	opts := []db.KVStoreFlusherOption{
		db.SerializeOption(func(wi *batch.WriteInfo) []byte {
			// current height is moved to the metadata namespace
			// transform it back for the purpose of calculating digest
			wi = rtf.transCurrentHeight(wi)
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

func (rtf *daoRTFArchive) transCurrentHeight(wi *batch.WriteInfo) *batch.WriteInfo {
	if wi.Namespace() == rtf.metaNS && string(wi.Key()) == CurrentHeightKey {
		return batch.NewWriteInfo(wi.WriteType(), AccountKVNamespace, wi.Key(), wi.Value(), wi.Error())
	}
	return wi
}
