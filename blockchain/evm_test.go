// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core-internal/blockchain/action"
	"github.com/iotexproject/iotex-core-internal/config"
	"github.com/iotexproject/iotex-core-internal/state"
	ta "github.com/iotexproject/iotex-core-internal/test/testaddress"
	"github.com/iotexproject/iotex-core-internal/testutil"
)

func TestEVM(t *testing.T) {
	fmt.Printf("Test EVM\n")
	require := require.New(t)
	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)
	testutil.CleanupPath(t, testDBPath)
	defer testutil.CleanupPath(t, testDBPath)

	ctx := context.Background()
	cfg := config.Default
	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	sf, err := state.NewFactory(&cfg, state.DefaultTrieOption())
	require.Nil(err)
	require.NoError(sf.Start(ctx))
	_, err = sf.LoadOrCreateState(ta.Addrinfo["producer"].RawAddress, Gen.TotalSupply)
	require.NoError(err)
	bc := NewBlockchain(&cfg, PrecreatedStateFactoryOption(sf), BoltDBDaoOption())
	defer func() {
		err := bc.Stop(ctx)
		require.NoError(err)
	}()
	require.NotNil(bc)
	fmt.Printf("Create a test execution\n")
	data, _ := hex.DecodeString("0x608060405234801561001057600080fd5b5060df8061001f6000396000f3006080604052600436106049576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff16806360fe47b114604e5780636d4ce63c146078575b600080fd5b348015605957600080fd5b5060766004803603810190808035906020019092919050505060a0565b005b348015608357600080fd5b50608a60aa565b6040518082815260200191505060405180910390f35b8060008190555050565b600080549050905600a165627a7a7230582002faabbefbbda99b20217cf33cb8ab8100caf1542bf1f48117d72e2c59139aea0029")
	execution, err := action.NewExecution(ta.Addrinfo["producer"].RawAddress, action.EmptyAddress, 1, big.NewInt(0), uint32(4000), uint32(10), data)
	require.NoError(err)
	execution, err = execution.Sign(ta.Addrinfo["producer"])
	require.NoError(err)
	blk, err := bc.MintNewBlock(nil, nil, []*action.Execution{execution}, ta.Addrinfo["producer"], "")
	fmt.Printf("Commit block\n")

	require.NoError(err)
	err = bc.CommitBlock(blk)
	fmt.Printf("Committed\n")
	/*
		TODO (zhi) check contract code
		contractAddr := "io1qyqsyqcy3kcd2pyfwus69nzgvkwhg8mk8h336dt86pg6cj"
		contractPubkeyHash, err := iotxaddress.GetPubkeyHash(contractAddr)
		fmt.Printf("pub key: %v, %v", contractPubkeyHash, err)
		contractState, err := sf.State(contractAddr)
		fmt.Printf("contract state: %+v, %v\n", contractState, err)
		require.NoError(err)
	*/
}
