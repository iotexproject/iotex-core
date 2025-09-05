// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package factory

import (
	"context"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
)

type daoRTF struct {
	lifecycle.Readiness
	dao db.KVStore
}

func newDaoRetrofitter(dao db.KVStore) *daoRTF {
	return &daoRTF{
		dao: dao,
	}
}

func (rtf *daoRTF) Start(ctx context.Context) error {
	if err := rtf.dao.Start(ctx); err != nil {
		return err
	}
	if err := rtf.TurnOn(); err != nil {
		return err
	}
	return nil
}

func (rtf *daoRTF) Stop(ctx context.Context) error {
	if !rtf.IsReady() {
		return nil
	}
	if err := rtf.TurnOff(); err != nil {
		return err
	}
	return rtf.dao.Stop(ctx)
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
