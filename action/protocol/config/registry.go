// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package config

import (
	"context"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/config/configpb"
	"github.com/iotexproject/iotex-core/state"
)

var registryKey = []byte("reg")

type activeProtocols struct {
	pids []string
}

// Serialize serializes active protocols into bytes
func (ap activeProtocols) Serialize() ([]byte, error) {
	gen := configpb.ActiveProtocols{
		Pids: ap.pids,
	}
	return proto.Marshal(&gen)
}

// Deserialize deserializes bytes into active protocols
func (ap *activeProtocols) Deserialize(data []byte) error {
	gen := configpb.ActiveProtocols{}
	if err := proto.Unmarshal(data, &gen); err != nil {
		return err
	}
	ap.pids = gen.Pids
	return nil
}

// UpdateActiveProtocols updates the active protocols
func (p *Protocol) UpdateActiveProtocols(
	ctx context.Context,
	sm protocol.StateManager,
	additions []string,
	removals []string,
) (*action.Log, error) {
	raCtx := protocol.MustGetRunActionsCtx(ctx)
	if err := p.assertAdmin(raCtx.Caller); err != nil {
		return nil, err
	}
	pids, err := p.ActiveProtocols(ctx, sm)
	if err != nil {
		return nil, err
	}
	registryLog := configpb.RegistryLog{
		Additions: make([]string, 0),
		Removals:  make([]string, 0),
	}
	// Add pids if they don't exist
	for _, pidi := range additions {
		exists := false
		for _, pidj := range pids {
			if pidi == pidj {
				exists = true
				break
			}
		}
		if !exists {
			registryLog.Additions = append(registryLog.Additions, pidi)
			pids = append(pids, pidi)
		}
	}
	// Remove pids if they exist
	for _, pidi := range removals {
		for j, pidj := range pids {
			if pidi == pidj {
				registryLog.Removals = append(registryLog.Removals, pidj)
				pids = append(pids[:j], pids[j+1:]...)
				break
			}
		}
	}
	// Put the active protocols
	if err := p.putState(sm, registryKey, &activeProtocols{
		pids: pids,
	}); err != nil {
		return nil, err
	}
	// Generate log
	data, err := proto.Marshal(&registryLog)
	if err != nil {
		return nil, err
	}
	return &action.Log{
		Address:     p.addr.String(),
		Topics:      nil,
		Data:        data,
		BlockHeight: raCtx.BlockHeight,
		ActionHash:  raCtx.ActionHash,
	}, nil
}

// ActiveProtocols returns the active protocols
func (p *Protocol) ActiveProtocols(ctx context.Context, sm protocol.StateManager) ([]string, error) {
	ap := activeProtocols{}
	err := p.state(sm, registryKey, &ap)
	if err == nil {
		return ap.pids, nil
	}
	if errors.Cause(err) == state.ErrStateNotExist {
		pids := make([]string, len(p.initActiveProtocols))
		copy(pids, p.initActiveProtocols)
		return pids, nil
	}
	return nil, err
}
