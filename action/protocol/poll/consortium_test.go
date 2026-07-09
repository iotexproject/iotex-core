// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package poll

import (
	"context"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

func TestNewConsortiumCommittee(t *testing.T) {
	r := require.New(t)

	t.Run("nil read contract is rejected", func(t *testing.T) {
		_, err := NewConsortiumCommittee(nil, nil, nil)
		r.Error(err)
		r.Contains(err.Error(), "empty read contract callback")
	})

	t.Run("success", func(t *testing.T) {
		p, err := NewConsortiumCommittee(nil, func(context.Context, string, []byte, bool) ([]byte, error) {
			return nil, nil
		}, nil)
		r.NoError(err)
		r.Equal(_protocolID, p.Name())
		cc, ok := p.(*consortiumCommittee)
		r.True(ok)
		r.NotNil(cc.addr)
		r.NotNil(cc.contractReader)
	})
}

func TestToEtherAddressSlice(t *testing.T) {
	r := require.New(t)
	addrs := []common.Address{
		common.BytesToAddress([]byte{0x1}),
		common.BytesToAddress([]byte{0x2}),
	}
	got, err := toEtherAddressSlice(addrs)
	r.NoError(err)
	r.Equal(addrs, got)

	_, err = toEtherAddressSlice("not-a-slice")
	r.ErrorIs(err, ErrWrongData)
}

func TestContractReaderFunc(t *testing.T) {
	r := require.New(t)
	called := false
	var f contractReaderFunc = func(_ context.Context, contract string, data []byte) ([]byte, error) {
		called = true
		r.Equal("contract-addr", contract)
		r.Equal([]byte{0x9}, data)
		return []byte{0xaa}, nil
	}
	out, err := f.Read(context.Background(), "contract-addr", []byte{0x9})
	r.NoError(err)
	r.Equal([]byte{0xaa}, out)
	r.True(called)
}

func TestGenContractReaderFromReadContract(t *testing.T) {
	r := require.New(t)
	var gotSetting bool
	rc := func(_ context.Context, contract string, data []byte, setting bool) ([]byte, error) {
		gotSetting = setting
		return []byte{0x1}, nil
	}
	reader := genContractReaderFromReadContract(rc, true)
	out, err := reader(context.Background(), "c", []byte{0x2})
	r.NoError(err)
	r.Equal([]byte{0x1}, out)
	r.True(gotSetting)
}

// readDelegatesWithContractReader packs a `delegates` call, hands off to the
// contractReader, and unpacks the resulting address list into candidates.
func TestReadDelegatesWithContractReader(t *testing.T) {
	r := require.New(t)

	parsedABI, err := abi.JSON(strings.NewReader(ConsortiumManagementABI))
	r.NoError(err)

	delegateEthAddrs := []common.Address{
		common.BytesToAddress(identityset.Address(1).Bytes()),
		common.BytesToAddress(identityset.Address(2).Bytes()),
	}
	packedResult, err := parsedABI.Methods["delegates"].Outputs.Pack(delegateEthAddrs)
	r.NoError(err)

	reads := 0
	cc := &consortiumCommittee{
		abi:      parsedABI,
		contract: "io1contract",
		contractReader: contractReaderFunc(func(_ context.Context, _ string, data []byte) ([]byte, error) {
			reads++
			// verify the packed call selector matches `delegates()`
			expected, perr := parsedABI.Pack("delegates")
			r.NoError(perr)
			r.Equal(expected, data)
			return packedResult, nil
		}),
	}

	// registry with rolldpos so GetEpochNum works
	registry := protocol.NewRegistry()
	r.NoError(registry.Register("rolldpos", rolldpos.NewProtocol(36, 36, 20)))
	ctx := protocol.WithRegistry(context.Background(), registry)
	ctx = protocol.WithBlockchainCtx(ctx, protocol.BlockchainCtx{
		Tip: protocol.TipInfo{Height: 1},
	})

	cands, err := cc.readDelegates(ctx)
	r.NoError(err)
	r.Len(cands, 2)
	r.Equal(identityset.Address(1).String(), cands[0].Address)
	r.Equal(identityset.Address(2).String(), cands[1].Address)
	r.Equal(1, reads)

	// second read in the same epoch must hit the epoch buffer and NOT call
	// the contract reader again.
	cands2, err := cc.readDelegates(ctx)
	r.NoError(err)
	r.Equal(cands, cands2)
	r.Equal(1, reads, "same-epoch read should be served from cache")

	// a read in a different epoch bypasses the cache and reads again.
	ctxNextEpoch := protocol.WithBlockchainCtx(
		protocol.WithRegistry(context.Background(), registry),
		protocol.BlockchainCtx{Tip: protocol.TipInfo{Height: 100_000}},
	)
	_, err = cc.readDelegates(ctxNextEpoch)
	r.NoError(err)
	r.Equal(2, reads, "new-epoch read should query the contract again")
}

func TestConsortiumCommitteeRegister(t *testing.T) {
	r := require.New(t)
	p, err := NewConsortiumCommittee(nil, func(context.Context, string, []byte, bool) ([]byte, error) {
		return nil, nil
	}, nil)
	r.NoError(err)
	cc := p.(*consortiumCommittee)

	registry := protocol.NewRegistry()
	r.NoError(cc.Register(registry))
	_, ok := registry.Find(_protocolID)
	r.True(ok)

	// ForceRegister replaces without error
	r.NoError(cc.ForceRegister(registry))
}
