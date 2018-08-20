// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"context"
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/blockchain/action"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	ta "github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestEVM(t *testing.T) {
	logger.Info().Msgf("Test EVM")
	require := require.New(t)
	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)
	testutil.CleanupPath(t, testDBPath)
	defer testutil.CleanupPath(t, testDBPath)

	ctx := context.Background()
	cfg := config.Default
	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	bc := NewBlockchain(&cfg, DefaultStateFactoryOption(), BoltDBDaoOption())
	require.NotNil(bc)
	defer func() {
		err := bc.Stop(ctx)
		require.NoError(err)
	}()
	_, err := bc.CreateState(ta.Addrinfo["producer"].RawAddress, Gen.TotalSupply)
	require.NoError(err)
	// data, _ := hex.DecodeString("6080604052600436106049576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff16806360fe47b114604e5780636d4ce63c146078575b600080fd5b348015605957600080fd5b5060766004803603810190808035906020019092919050505060a0565b005b348015608357600080fd5b50608a60aa565b6040518082815260200191505060405180910390f35b8060008190555050565b600080549050905600a165627a7a723058202b8e3ee299d6212c404a3f109eb874d5af929b6d2d701819421e3686c4c82fbd0029")
	data, _ := hex.DecodeString("608060405234801561001057600080fd5b5060df8061001f6000396000f3006080604052600436106049576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff16806360fe47b114604e5780636d4ce63c146078575b600080fd5b348015605957600080fd5b5060766004803603810190808035906020019092919050505060a0565b005b348015608357600080fd5b50608a60aa565b6040518082815260200191505060405180910390f35b8060008190555050565b600080549050905600a165627a7a7230582002faabbefbbda99b20217cf33cb8ab8100caf1542bf1f48117d72e2c59139aea0029")
	// data, _ := hex.DecodeString("6060604052600436106100565763ffffffff7c010000000000000000000000000000000000000000000000000000000060003504166341c0e1b581146100585780637bf786f81461006b578063fbf788d61461009c575b005b341561006357600080fd5b6100566100ca565b341561007657600080fd5b61008a600160a060020a03600435166100f1565b60405190815260200160405180910390f35b34156100a757600080fd5b610056600160a060020a036004351660243560ff60443516606435608435610103565b60005433600160a060020a03908116911614156100ef57600054600160a060020a0316ff5b565b60016020526000908152604090205481565b600160a060020a0385166000908152600160205260408120548190861161012957600080fd5b3087876040516c01000000000000000000000000600160a060020a03948516810282529290931690910260148301526028820152604801604051809103902091506001828686866040516000815260200160405260006040516020015260405193845260ff90921660208085019190915260408085019290925260608401929092526080909201915160208103908084039060008661646e5a03f115156101cf57600080fd5b505060206040510351600054600160a060020a039081169116146101f257600080fd5b50600160a060020a03808716600090815260016020526040902054860390301631811161026257600160a060020a0387166000818152600160205260409081902088905582156108fc0290839051600060405180830381858888f19350505050151561025d57600080fd5b6102b7565b6000547f2250e2993c15843b32621c89447cc589ee7a9f049c026986e545d3c2c0c6f97890600160a060020a0316604051600160a060020a03909116815260200160405180910390a186600160a060020a0316ff5b505050505050505600a165627a7a72305820533e856fc37e3d64d1706bcc7dfb6b1d490c8d566ea498d9d01ec08965a896ca0029")
	execution, err := action.NewExecution(
		ta.Addrinfo["producer"].RawAddress, action.EmptyAddress, 1, big.NewInt(0), uint32(100000), uint32(10), data)
	require.NoError(err)
	execution, err = execution.Sign(ta.Addrinfo["producer"])
	require.NoError(err)
	blk, err := bc.MintNewBlock(nil, nil, []*action.Execution{execution}, ta.Addrinfo["producer"], "")
	require.NoError(err)
	require.Nil(bc.CommitBlock(blk))

	h, _ := hex.DecodeString("8db0d504897721a2cc48659d741f763de31d3567")
	contractAddrHash := byteutil.BytesTo20B(h)
	code, err := bc.GetFactory().GetCode(contractAddrHash)
	require.Nil(err)
	require.Equal(data[31:], code)

	// store to key 0
	contractAddr := "io1qyqsyqcy3kcd2pyfwus69nzgvkwhg8mk8h336dt86pg6cj"
	data, _ = hex.DecodeString("60fe47b1000000000000000000000000000000000000000000000000000000000000000f")
	execution, err = action.NewExecution(
		ta.Addrinfo["producer"].RawAddress, contractAddr, 2, big.NewInt(0), uint32(120000), uint32(10), data)
	require.NoError(err)
	execution, err = execution.Sign(ta.Addrinfo["producer"])
	logger.Info().Msgf("execution %+v", execution)
	require.NoError(err)
	blk, err = bc.MintNewBlock(nil, nil, []*action.Execution{execution}, ta.Addrinfo["producer"], "")
	require.NoError(err)
	require.Nil(bc.CommitBlock(blk))

	v, err := bc.GetFactory().GetContractState(contractAddrHash, hash.ZeroHash32B)
	require.Nil(err)
	require.Equal(byte(15), v[31])

	// read from key 0
	contractAddr = "io1qyqsyqcy3kcd2pyfwus69nzgvkwhg8mk8h336dt86pg6cj"
	data, _ = hex.DecodeString("6d4ce63c")
	execution, err = action.NewExecution(
		ta.Addrinfo["producer"].RawAddress, contractAddr, 3, big.NewInt(0), uint32(120000), uint32(10), data)
	require.NoError(err)
	execution, err = execution.Sign(ta.Addrinfo["producer"])
	logger.Info().Msgf("execution %+v", execution)
	require.NoError(err)
	blk, err = bc.MintNewBlock(nil, nil, []*action.Execution{execution}, ta.Addrinfo["producer"], "")
	require.NoError(err)
	require.Nil(bc.CommitBlock(blk))
}

func TestLogReceipt(t *testing.T) {
	require := require.New(t)
	log := Log{Address: "abcde", Data: []byte("12345"), BlockNumber: 5, Index: 6}
	var topic hash.Hash32B
	copy(topic[:], hash.Hash256b([]byte("12345")))
	log.Topics = []hash.Hash32B{topic}
	copy(log.TxnHash[:], hash.Hash256b([]byte("11111")))
	copy(log.BLockHash[:], hash.Hash256b([]byte("22222")))
	s, err := log.Serialize()
	require.NoError(err)
	actuallog := Log{}
	actuallog.Deserialize(s)
	require.Equal(log.Address, actuallog.Address)
	require.Equal(log.Topics[0], actuallog.Topics[0])
	require.Equal(len(log.Topics), len(actuallog.Topics))
	require.Equal(log.Data, actuallog.Data)
	require.Equal(log.BlockNumber, actuallog.BlockNumber)
	require.Equal(log.TxnHash, actuallog.TxnHash)
	require.Equal(log.BLockHash, actuallog.BLockHash)
	require.Equal(log.Index, actuallog.Index)

	receipt := Receipt{ReturnValue: []byte("12345"), Status: 5, GasConsumed: 6, ContractAddress: "aaaaa", Logs: []*Log{&log}}
	copy(receipt.Hash[:], hash.Hash256b([]byte("33333")))
	s, err = receipt.Serialize()
	require.NoError(err)
	actualReceipt := Receipt{}
	actualReceipt.Deserialize(s)
	require.Equal(receipt.ReturnValue, actualReceipt.ReturnValue)
	require.Equal(receipt.Status, actualReceipt.Status)
	require.Equal(receipt.GasConsumed, actualReceipt.GasConsumed)
	require.Equal(receipt.ContractAddress, actualReceipt.ContractAddress)
	require.Equal(receipt.Logs[0], actualReceipt.Logs[0])
	require.Equal(len(receipt.Logs), len(actualReceipt.Logs))
	require.Equal(receipt.Hash, actualReceipt.Hash)
}

func TestRollDice(t *testing.T) {
	logger.Info().Msgf("Test Roll Dice\n")
	require := require.New(t)
	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)
	testutil.CleanupPath(t, testDBPath)
	defer testutil.CleanupPath(t, testDBPath)

	ctx := context.Background()
	cfg := config.Default
	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	bc := NewBlockchain(&cfg, DefaultStateFactoryOption(), BoltDBDaoOption())
	require.NotNil(bc)
	defer func() {
		err := bc.Stop(ctx)
		require.NoError(err)
	}()
	_, err := bc.CreateState(ta.Addrinfo["producer"].RawAddress, Gen.TotalSupply)
	require.NoError(err)
	data, _ := hex.DecodeString("608060405234801561001057600080fd5b5061037a806100206000396000f3006080604052600436106100565763ffffffff7c01000000000000000000000000000000000000000000000000000000006000350416631e67bed8811461005b5780638965356f146100d1578063b214faa51461012a575b600080fd5b6040805160206004803580820135601f81018490048402850184019095528484526100bf9436949293602493928401919081908401838280828437509497505050923573ffffffffffffffffffffffffffffffffffffffff16935061013792505050565b60408051918252519081900360200190f35b3480156100dd57600080fd5b506040805160206004803580820135601f81018490048402850184019095528484526100bf9436949293602493928401919081908401838280828437509497506101969650505050505050565b6101356004356102ac565b005b60008061014384610196565b60405190915073ffffffffffffffffffffffffffffffffffffffff8416906305f5e100830280156108fc02916000818181858888f1935050505015801561018e573d6000803e3d6000fd5b505092915050565b6000600561029c6002448560014303406040516020018084815260200183805190602001908083835b602083106101de5780518252601f1990920191602091820191016101bf565b51815160209384036101000a60001901801990921691161790529201938452506040805180850381529382019081905283519395509350839290850191508083835b6020831061023f5780518252601f199092019160209182019101610220565b51815160209384036101000a600019018019909216911617905260405191909301945091925050808303816000865af1158015610280573d6000803e3d6000fd5b5050506040513d602081101561029557600080fd5b50516102e6565b8115156102a557fe5b0692915050565b604080513481529051829133917f19dacbf83c5de6658e14cbf7bcae5c15eca2eedecf1c66fbca928e4d351bea0f9181900360200190a350565b600080805b60208110156103475780600101602060ff160360080260020a848260208110151561031257fe5b7f010000000000000000000000000000000000000000000000000000000000000091901a8102040291909101906001016102eb565b50929150505600a165627a7a723058202aeab982e3a92e89ae0fcf96fabb5402d9130c00d1531abbe483b8360a29a8580029")
	execution, err := action.NewExecution(
		ta.Addrinfo["producer"].RawAddress, action.EmptyAddress, 1, big.NewInt(0), uint32(1000000), uint32(10), data)
	require.NoError(err)
	execution, err = execution.Sign(ta.Addrinfo["producer"])
	require.NoError(err)
	blk, err := bc.MintNewBlock(nil, nil, []*action.Execution{execution}, ta.Addrinfo["producer"], "")
	require.NoError(err)
	require.Nil(bc.CommitBlock(blk))

	logger.Info().Msg("Deposit to contract")
	contractAddr := "io1qyqsyqcy3kcd2pyfwus69nzgvkwhg8mk8h336dt86pg6cj"
	data, _ = hex.DecodeString("b214faa5abf43c4cec080bae7ec91f1a101efaba1dea724b78480ab620a4e482e0d85707")
	execution, err = action.NewExecution(
		ta.Addrinfo["producer"].RawAddress, contractAddr, 2, big.NewInt(500000000), uint32(120000), uint32(10), data)
	require.NoError(err)
	execution, err = execution.Sign(ta.Addrinfo["producer"])
	logger.Info().Msgf("execution %+v", execution)
	require.NoError(err)
	blk, err = bc.MintNewBlock(nil, nil, []*action.Execution{execution}, ta.Addrinfo["producer"], "")
	require.NoError(err)
	require.Nil(bc.CommitBlock(blk))

	balance, err := bc.Balance(contractAddr)
	require.NoError(err)
	require.Equal(0, balance.Cmp(big.NewInt(500000000)))

	logger.Info().Msg("Roll Dice")
	data, _ = hex.DecodeString("1e67bed80000000000000000000000000000000000000000000000000000000000000040000000000000000000000000fd99ea5ad63d9d3a8a4d614bcae138069502255800000000000000000000000000000000000000000000000000000000000000033132330000000000000000000000000000000000000000000000000000000000")
	execution, err = action.NewExecution(
		ta.Addrinfo["producer"].RawAddress, contractAddr, 2, big.NewInt(0), uint32(120000), uint32(10), data)
	require.NoError(err)
	execution, err = execution.Sign(ta.Addrinfo["producer"])
	logger.Info().Msgf("execution %+v\n", execution)
	require.NoError(err)
	blk, err = bc.MintNewBlock(nil, nil, []*action.Execution{execution}, ta.Addrinfo["producer"], "")
	require.NoError(err)
	require.Nil(bc.CommitBlock(blk))
	logger.Info().Msg("Done")

	balance, err = bc.Balance(ta.Addrinfo["alfa"].RawAddress)
	require.NoError(err)
	require.Equal(0, balance.Cmp(big.NewInt(300000000)))

	balance, err = bc.Balance(contractAddr)
	require.NoError(err)
	logger.Info().Msgf("balance: %v\n", balance)
	require.Equal(0, balance.Cmp(big.NewInt(200000000)))
}
