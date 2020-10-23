// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package factory

import (
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/state"
)

// historyStateReader implements state reader interface, wrap factory with archive height
type historyStateReader struct {
	height uint64
	sf     Factory
}

// NewHistoryStateReader creates new history state reader by given state factory and height
func NewHistoryStateReader(sf Factory, h uint64) protocol.StateReader {
	return &historyStateReader{
		sf:     sf,
		height: h,
	}
}

// Height returns archive height
func (hReader *historyStateReader) Height() (uint64, error) {
	return hReader.height, nil
}

// State returns history state in the archive mode state factory
func (hReader *historyStateReader) State(s interface{}, opts ...protocol.StateOption) (uint64, error) {
	return hReader.height, hReader.sf.StateAtHeight(hReader.height, s, opts...)
}

// States returns history states in the archive mode state factory
func (hReader *historyStateReader) States(opts ...protocol.StateOption) (uint64, state.Iterator, error) {
	iterator, err := hReader.sf.StatesAtHeight(hReader.height, opts...)
	if err != nil {
		return 0, nil, err
	}
	return hReader.height, iterator, nil
}

// ReadView reads the view
func (hReader *historyStateReader) ReadView(name string) (interface{}, error) {
	return hReader.sf.ReadView(name)
}
