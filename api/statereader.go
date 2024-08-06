package api

import (
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/state/factory"
)

type stateReaderWithHeight struct {
	f      factory.Factory
	height uint64
}

func newStateReaderWithHeight(f factory.Factory, height uint64) *stateReaderWithHeight {
	return &stateReaderWithHeight{f: f, height: height}
}

func (sr *stateReaderWithHeight) State(obj interface{}, opts ...protocol.StateOption) (uint64, error) {
	return sr.height, sr.f.StateAtHeight(sr.height, obj, opts...)
}

func (sr *stateReaderWithHeight) States(opts ...protocol.StateOption) (uint64, state.Iterator, error) {
	iter, err := sr.f.StatesAtHeight(sr.height, opts...)
	return sr.height, iter, err
}

func (sr *stateReaderWithHeight) Height() (uint64, error) {
	return sr.height, nil
}

func (sr *stateReaderWithHeight) ReadView(string) (interface{}, error) {
	return nil, errors.New(" ReadView is not supported in stateReaderWithHeight")
}
