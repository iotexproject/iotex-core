// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/systemcontracts"
)

// Encodes / Decodes implement the kvListContainer interface used by erigon
// archive storage. lastExitEpoch is a single-valued state kept under the
// CandsMap namespace alongside the candidate-map kvList, so when archive is
// enabled it flows through kvListStorage and needs this shape. We collapse
// the single value into a one-entry list with an empty suffix key.
//
// Kept in a dedicated file so adding these methods doesn't shift the
// compiled layout of protocol.go — gomonkey-based tests there patch by
// function offset and break when this file changes the offset table.
func (le *lastExitEpoch) Encodes() ([][]byte, []systemcontracts.GenericValue, error) {
	data, err := le.Serialize()
	if err != nil {
		return nil, nil, err
	}
	return [][]byte{nil}, []systemcontracts.GenericValue{{PrimaryData: data}}, nil
}

// Decodes restores lastExitEpoch from the single-entry list produced by
// Encodes (or by a previous Store via kvListStorage).
func (le *lastExitEpoch) Decodes(_ [][]byte, values []systemcontracts.GenericValue) error {
	if len(values) == 0 {
		return errors.New("no data to decode last exit epoch")
	}
	return le.Deserialize(values[0].PrimaryData)
}
