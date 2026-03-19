package factory

import (
	"context"

	"github.com/iotexproject/iotex-core/v2/state/factory/erigonstore"
	"github.com/pkg/errors"
)

// erigonWorkingSetStoreForSimulate is a working set store that uses erigon as the main store
// It is used for simulating transactions without actually committing them to the erigon store.
type erigonWorkingSetStoreForSimulate struct {
	*erigonstore.ErigonWorkingSetStore
	base workingSetStore
}

func newErigonWorkingSetStoreForSimulate(erigonStore *erigonstore.ErigonWorkingSetStore, base workingSetStore) *erigonWorkingSetStoreForSimulate {
	return &erigonWorkingSetStoreForSimulate{
		ErigonWorkingSetStore: erigonStore,
		base:                  base,
	}
}

func (store *erigonWorkingSetStoreForSimulate) Commit(context.Context, uint64) error {
	return errors.New("commit is not supported in erigonWorkingSetStoreForSimulate")
}
