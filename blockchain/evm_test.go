// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/blockchain/action"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/iotxaddress"
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
	cfg.Explorer.Enabled = true
	bc := NewBlockchain(&cfg, DefaultStateFactoryOption(), BoltDBDaoOption())
	require.NoError(bc.Start(ctx))
	require.NotNil(bc)
	defer func() {
		err := bc.Stop(ctx)
		require.NoError(err)
	}()
	_, err := bc.CreateState(ta.Addrinfo["producer"].RawAddress, Gen.TotalSupply)
	bc.GetFactory().CommitStateChanges(0, nil, nil, nil)
	require.NoError(err)
	// data, _ := hex.DecodeString("6080604052600436106049576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff16806360fe47b114604e5780636d4ce63c146078575b600080fd5b348015605957600080fd5b5060766004803603810190808035906020019092919050505060a0565b005b348015608357600080fd5b50608a60aa565b6040518082815260200191505060405180910390f35b8060008190555050565b600080549050905600a165627a7a723058202b8e3ee299d6212c404a3f109eb874d5af929b6d2d701819421e3686c4c82fbd0029")
	data, _ := hex.DecodeString("608060405234801561001057600080fd5b5060df8061001f6000396000f3006080604052600436106049576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff16806360fe47b114604e5780636d4ce63c146078575b600080fd5b348015605957600080fd5b5060766004803603810190808035906020019092919050505060a0565b005b348015608357600080fd5b50608a60aa565b6040518082815260200191505060405180910390f35b8060008190555050565b600080549050905600a165627a7a7230582002faabbefbbda99b20217cf33cb8ab8100caf1542bf1f48117d72e2c59139aea0029")
	// data, _ := hex.DecodeString("6060604052600436106100565763ffffffff7c010000000000000000000000000000000000000000000000000000000060003504166341c0e1b581146100585780637bf786f81461006b578063fbf788d61461009c575b005b341561006357600080fd5b6100566100ca565b341561007657600080fd5b61008a600160a060020a03600435166100f1565b60405190815260200160405180910390f35b34156100a757600080fd5b610056600160a060020a036004351660243560ff60443516606435608435610103565b60005433600160a060020a03908116911614156100ef57600054600160a060020a0316ff5b565b60016020526000908152604090205481565b600160a060020a0385166000908152600160205260408120548190861161012957600080fd5b3087876040516c01000000000000000000000000600160a060020a03948516810282529290931690910260148301526028820152604801604051809103902091506001828686866040516000815260200160405260006040516020015260405193845260ff90921660208085019190915260408085019290925260608401929092526080909201915160208103908084039060008661646e5a03f115156101cf57600080fd5b505060206040510351600054600160a060020a039081169116146101f257600080fd5b50600160a060020a03808716600090815260016020526040902054860390301631811161026257600160a060020a0387166000818152600160205260409081902088905582156108fc0290839051600060405180830381858888f19350505050151561025d57600080fd5b6102b7565b6000547f2250e2993c15843b32621c89447cc589ee7a9f049c026986e545d3c2c0c6f97890600160a060020a0316604051600160a060020a03909116815260200160405180910390a186600160a060020a0316ff5b505050505050505600a165627a7a72305820533e856fc37e3d64d1706bcc7dfb6b1d490c8d566ea498d9d01ec08965a896ca0029")
	execution, err := action.NewExecution(
		ta.Addrinfo["producer"].RawAddress, action.EmptyAddress, 1, big.NewInt(0), uint64(100000), big.NewInt(10), data)
	require.NoError(err)
	execution, err = execution.Sign(ta.Addrinfo["producer"])
	require.NoError(err)
	blk, err := bc.MintNewBlock(nil, nil, []*action.Execution{execution}, ta.Addrinfo["producer"], "")
	require.NoError(err)
	require.Nil(bc.CommitBlock(blk))
	require.Equal(1, len(blk.receipts))

	eHash := execution.Hash()
	r, _ := bc.GetReceiptByExecutionHash(eHash)
	require.Equal(eHash, r.Hash)
	h, _ := iotxaddress.GetPubkeyHash(r.ContractAddress)
	contractAddrHash := byteutil.BytesTo20B(h)
	code, err := bc.GetFactory().GetCode(contractAddrHash)
	require.Nil(err)
	require.Equal(data[31:], code)

	exe, err := bc.GetExecutionByExecutionHash(eHash)
	require.Nil(err)
	require.Equal(eHash, exe.Hash())

	exes, err := bc.GetExecutionsFromAddress(ta.Addrinfo["producer"].RawAddress)
	require.Nil(err)
	require.Equal(1, len(exes))
	require.Equal(eHash, exes[0])

	blkHash, err := bc.GetBlockHashByExecutionHash(eHash)
	require.Nil(err)
	require.Equal(blk.HashBlock(), blkHash)

	// store to key 0
	contractAddr := "io1qyqsyqcy3kcd2pyfwus69nzgvkwhg8mk8h336dt86pg6cj"
	data, _ = hex.DecodeString("60fe47b1000000000000000000000000000000000000000000000000000000000000000f")
	execution, err = action.NewExecution(
		ta.Addrinfo["producer"].RawAddress, contractAddr, 2, big.NewInt(0), uint64(120000), big.NewInt(10), data)
	require.NoError(err)
	execution, err = execution.Sign(ta.Addrinfo["producer"])
	logger.Info().Msgf("execution %+v", execution)
	require.NoError(err)
	blk, err = bc.MintNewBlock(nil, nil, []*action.Execution{execution}, ta.Addrinfo["producer"], "")
	require.NoError(err)
	require.Nil(bc.CommitBlock(blk))
	require.Equal(1, len(blk.receipts))

	v, err := bc.GetFactory().GetContractState(contractAddrHash, hash.ZeroHash32B)
	require.Nil(err)
	require.Equal(byte(15), v[31])

	eHash = execution.Hash()
	r, _ = bc.GetReceiptByExecutionHash(eHash)
	require.Equal(eHash, r.Hash)

	// read from key 0
	contractAddr = "io1qyqsyqcy3kcd2pyfwus69nzgvkwhg8mk8h336dt86pg6cj"
	data, _ = hex.DecodeString("6d4ce63c")
	execution, err = action.NewExecution(
		ta.Addrinfo["producer"].RawAddress, contractAddr, 3, big.NewInt(0), uint64(120000), big.NewInt(10), data)
	require.NoError(err)
	execution, err = execution.Sign(ta.Addrinfo["producer"])
	logger.Info().Msgf("execution %+v", execution)
	require.NoError(err)
	blk, err = bc.MintNewBlock(nil, nil, []*action.Execution{execution}, ta.Addrinfo["producer"], "")
	require.NoError(err)
	require.Nil(bc.CommitBlock(blk))
	require.Equal(1, len(blk.receipts))

	eHash = execution.Hash()
	r, _ = bc.GetReceiptByExecutionHash(eHash)
	require.Equal(eHash, r.Hash)
}

func TestLogReceipt(t *testing.T) {
	require := require.New(t)
	log := Log{Address: "abcde", Data: []byte("12345"), BlockNumber: 5, Index: 6}
	var topic hash.Hash32B
	copy(topic[:], hash.Hash256b([]byte("12345")))
	log.Topics = []hash.Hash32B{topic}
	copy(log.TxnHash[:], hash.Hash256b([]byte("11111")))
	copy(log.BlockHash[:], hash.Hash256b([]byte("22222")))
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
	require.Equal(log.BlockHash, actuallog.BlockHash)
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
	logger.Warn().Msg("======= Test RollDice")
	require := require.New(t)
	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)
	testutil.CleanupPath(t, testDBPath)
	defer testutil.CleanupPath(t, testDBPath)

	ctx := context.Background()
	cfg := config.Default
	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	cfg.Explorer.Enabled = true
	bc := NewBlockchain(&cfg, DefaultStateFactoryOption(), BoltDBDaoOption())
	require.NoError(bc.Start(ctx))
	require.NotNil(bc)
	defer func() {
		err := bc.Stop(ctx)
		require.NoError(err)
	}()
	_, err := bc.CreateState(ta.Addrinfo["producer"].RawAddress, Gen.TotalSupply)
	require.NoError(err)
	_, err = bc.CreateState(ta.Addrinfo["alfa"].RawAddress, 0)
	require.NoError(err)
	_, err = bc.CreateState(ta.Addrinfo["bravo"].RawAddress, 12000000)
	require.NoError(err)
	bc.GetFactory().CommitStateChanges(0, nil, nil, nil)
	data, _ := hex.DecodeString("608060405234801561001057600080fd5b506102f5806100206000396000f3006080604052600436106100615763ffffffff7c01000000000000000000000000000000000000000000000000000000006000350416632885ad2c8114610066578063797d9fbd14610070578063cd5e3c5d14610091578063d0e30db0146100b8575b600080fd5b61006e6100c0565b005b61006e73ffffffffffffffffffffffffffffffffffffffff600435166100cb565b34801561009d57600080fd5b506100a6610159565b60408051918252519081900360200190f35b61006e610229565b6100c9336100cb565b565b60006100d5610159565b6040805182815290519192507fbae72e55df73720e0f671f4d20a331df0c0dc31092fda6c573f35ff7f37f283e919081900360200190a160405173ffffffffffffffffffffffffffffffffffffffff8316906305f5e100830280156108fc02916000818181858888f19350505050158015610154573d6000803e3d6000fd5b505050565b604080514460208083019190915260001943014082840152825180830384018152606090920192839052815160009360059361021a9360029391929182918401908083835b602083106101bd5780518252601f19909201916020918201910161019e565b51815160209384036101000a600019018019909216911617905260405191909301945091925050808303816000865af11580156101fe573d6000803e3d6000fd5b5050506040513d602081101561021357600080fd5b5051610261565b81151561022357fe5b06905090565b60408051348152905133917fe1fffcc4923d04b559f4d29a8bfc6cda04eb5b0d3c460751c2402c5c5cc9109c919081900360200190a2565b600080805b60208110156102c25780600101602060ff160360080260020a848260208110151561028d57fe5b7f010000000000000000000000000000000000000000000000000000000000000091901a810204029190910190600101610266565b50929150505600a165627a7a72305820a426929891673b0a04d7163b60113d28e7d0f48ea667680ba48126c182b872c10029")
	execution, err := action.NewExecution(
		ta.Addrinfo["producer"].RawAddress, action.EmptyAddress, 1, big.NewInt(0), uint64(1000000), big.NewInt(10), data)
	require.NoError(err)
	execution, err = execution.Sign(ta.Addrinfo["producer"])
	require.NoError(err)
	blk, err := bc.MintNewBlock(nil, nil, []*action.Execution{execution}, ta.Addrinfo["producer"], "")
	require.NoError(err)
	require.Nil(bc.CommitBlock(blk))

	logger.Warn().Msg("======= Deposit to contract")
	eHash := execution.Hash()
	r, _ := bc.GetReceiptByExecutionHash(eHash)
	require.Equal(eHash, r.Hash)
	contractAddr := r.ContractAddress
	data, _ = hex.DecodeString("d0e30db0")
	execution, err = action.NewExecution(
		ta.Addrinfo["producer"].RawAddress, contractAddr, 2, big.NewInt(500000000), uint64(120000), big.NewInt(10), data)
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
	data, _ = hex.DecodeString("797d9fbd000000000000000000000000fd99ea5ad63d9d3a8a4d614bcae1380695022558")
	execution, err = action.NewExecution(
		ta.Addrinfo["producer"].RawAddress, contractAddr, 3, big.NewInt(0), uint64(120000), big.NewInt(10), data)
	require.NoError(err)
	execution, err = execution.Sign(ta.Addrinfo["producer"])
	logger.Info().Msgf("execution %+v\n", execution)
	require.NoError(err)
	blk, err = bc.MintNewBlock(nil, nil, []*action.Execution{execution}, ta.Addrinfo["producer"], "")
	require.NoError(err)
	require.Nil(bc.CommitBlock(blk))

	// verify balance
	balance, err = bc.Balance(ta.Addrinfo["alfa"].RawAddress)
	require.NoError(err)
	require.Equal(0, balance.Cmp(big.NewInt(100000000)))

	balance, err = bc.Balance(contractAddr)
	require.NoError(err)
	require.Equal(0, balance.Cmp(big.NewInt(400000000)))

	logger.Info().Msg("Roll Dice To Self")
	balance, err = bc.Balance(ta.Addrinfo["bravo"].RawAddress)
	require.NoError(err)
	data, _ = hex.DecodeString("2885ad2c")
	execution, err = action.NewExecution(
		ta.Addrinfo["bravo"].RawAddress, contractAddr, 1, big.NewInt(0), uint64(120000), big.NewInt(10), data)
	require.NoError(err)
	execution, err = execution.Sign(ta.Addrinfo["bravo"])
	logger.Info().Msgf("execution %+v\n", execution)
	require.NoError(err)
	blk, err = bc.MintNewBlock(nil, nil, []*action.Execution{execution}, ta.Addrinfo["producer"], "")
	require.NoError(err)
	require.Nil(bc.CommitBlock(blk))
	balance, err = bc.Balance(ta.Addrinfo["bravo"].RawAddress)
	require.NoError(err)
	logger.Info().Msgf("balance: %d", balance)
	require.Equal(0, balance.Cmp(big.NewInt(12000000+100000000-274950)))
}

func TestERC20(t *testing.T) {
	require := require.New(t)
	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)
	testutil.CleanupPath(t, testDBPath)
	defer testutil.CleanupPath(t, testDBPath)

	ctx := context.Background()
	cfg := config.Default
	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	cfg.Explorer.Enabled = true
	bc := NewBlockchain(&cfg, DefaultStateFactoryOption(), BoltDBDaoOption())
	require.NoError(bc.Start(ctx))
	require.NotNil(bc)
	defer func() {
		err := bc.Stop(ctx)
		require.NoError(err)
	}()
	_, err := bc.CreateState(ta.Addrinfo["producer"].RawAddress, Gen.TotalSupply)
	require.NoError(err)
	_, err = bc.CreateState(ta.Addrinfo["alfa"].RawAddress, 0)
	require.NoError(err)
	_, err = bc.CreateState(ta.Addrinfo["bravo"].RawAddress, 0)
	require.NoError(err)
	bc.GetFactory().CommitStateChanges(0, nil, nil, nil)
	//data, _ := hex.DecodeString("608060405234801561001057600080fd5b5060df8061001f6000396000f3006080604052600436106049576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff16806360fe47b114604e5780636d4ce63c146078575b600080fd5b348015605957600080fd5b5060766004803603810190808035906020019092919050505060a0565b005b348015608357600080fd5b50608a60aa565b6040518082815260200191505060405180910390f35b8060008190555050565b600080549050905600a165627a7a7230582002faabbefbbda99b20217cf33cb8ab8100caf1542bf1f48117d72e2c59139aea0029")
	data, _ := hex.DecodeString("60806040526000600360146101000a81548160ff02191690831515021790555034801561002b57600080fd5b506040516020806119938339810180604052810190808051906020019092919050505033600360006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555080600181905550806000803373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020819055503373ffffffffffffffffffffffffffffffffffffffff16600073ffffffffffffffffffffffffffffffffffffffff167fddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef836040518082815260200191505060405180910390a3506118448061014f6000396000f3006080604052600436106100e6576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff16806306fdde03146100eb578063095ea7b31461017b57806318160ddd146101e057806323b872dd1461020b578063313ce567146102905780633f4ba83a146102c15780635c975abb146102d8578063661884631461030757806370a082311461036c5780638456cb59146103c35780638da5cb5b146103da57806395d89b4114610431578063a9059cbb146104c1578063d73dd62314610526578063dd62ed3e1461058b578063f2fde38b14610602575b600080fd5b3480156100f757600080fd5b50610100610645565b6040518080602001828103825283818151815260200191508051906020019080838360005b83811015610140578082015181840152602081019050610125565b50505050905090810190601f16801561016d5780820380516001836020036101000a031916815260200191505b509250505060405180910390f35b34801561018757600080fd5b506101c6600480360381019080803573ffffffffffffffffffffffffffffffffffffffff1690602001909291908035906020019092919050505061067e565b604051808215151515815260200191505060405180910390f35b3480156101ec57600080fd5b506101f56106ae565b6040518082815260200191505060405180910390f35b34801561021757600080fd5b50610276600480360381019080803573ffffffffffffffffffffffffffffffffffffffff169060200190929190803573ffffffffffffffffffffffffffffffffffffffff169060200190929190803590602001909291905050506106b8565b604051808215151515815260200191505060405180910390f35b34801561029c57600080fd5b506102a5610763565b604051808260ff1660ff16815260200191505060405180910390f35b3480156102cd57600080fd5b506102d6610768565b005b3480156102e457600080fd5b506102ed610828565b604051808215151515815260200191505060405180910390f35b34801561031357600080fd5b50610352600480360381019080803573ffffffffffffffffffffffffffffffffffffffff1690602001909291908035906020019092919050505061083b565b604051808215151515815260200191505060405180910390f35b34801561037857600080fd5b506103ad600480360381019080803573ffffffffffffffffffffffffffffffffffffffff16906020019092919050505061086b565b6040518082815260200191505060405180910390f35b3480156103cf57600080fd5b506103d86108b3565b005b3480156103e657600080fd5b506103ef610974565b604051808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200191505060405180910390f35b34801561043d57600080fd5b5061044661099a565b6040518080602001828103825283818151815260200191508051906020019080838360005b8381101561048657808201518184015260208101905061046b565b50505050905090810190601f1680156104b35780820380516001836020036101000a031916815260200191505b509250505060405180910390f35b3480156104cd57600080fd5b5061050c600480360381019080803573ffffffffffffffffffffffffffffffffffffffff169060200190929190803590602001909291905050506109d3565b604051808215151515815260200191505060405180910390f35b34801561053257600080fd5b50610571600480360381019080803573ffffffffffffffffffffffffffffffffffffffff16906020019092919080359060200190929190505050610a7c565b604051808215151515815260200191505060405180910390f35b34801561059757600080fd5b506105ec600480360381019080803573ffffffffffffffffffffffffffffffffffffffff169060200190929190803573ffffffffffffffffffffffffffffffffffffffff169060200190929190505050610aac565b6040518082815260200191505060405180910390f35b34801561060e57600080fd5b50610643600480360381019080803573ffffffffffffffffffffffffffffffffffffffff169060200190929190505050610b33565b005b6040805190810160405280600d81526020017f496f546558204e6574776f726b0000000000000000000000000000000000000081525081565b6000600360149054906101000a900460ff1615151561069c57600080fd5b6106a68383610c8b565b905092915050565b6000600154905090565b6000600360149054906101000a900460ff161515156106d657600080fd5b82600073ffffffffffffffffffffffffffffffffffffffff168173ffffffffffffffffffffffffffffffffffffffff161415151561071357600080fd5b3073ffffffffffffffffffffffffffffffffffffffff168173ffffffffffffffffffffffffffffffffffffffff161415151561074e57600080fd5b610759858585610d7d565b9150509392505050565b601281565b600360009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff161415156107c457600080fd5b600360149054906101000a900460ff1615156107df57600080fd5b6000600360146101000a81548160ff0219169083151502179055507f7805862f689e2f13df9f062ff482ad3ad112aca9e0847911ed832e158c525b3360405160405180910390a1565b600360149054906101000a900460ff1681565b6000600360149054906101000a900460ff1615151561085957600080fd5b6108638383611137565b905092915050565b60008060008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020549050919050565b600360009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff1614151561090f57600080fd5b600360149054906101000a900460ff1615151561092b57600080fd5b6001600360146101000a81548160ff0219169083151502179055507f6985a02210a168e66602d3235cb6db0e70f92b3ba4d376a33c0f3d9434bff62560405160405180910390a1565b600360009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1681565b6040805190810160405280600481526020017f494f54580000000000000000000000000000000000000000000000000000000081525081565b6000600360149054906101000a900460ff161515156109f157600080fd5b82600073ffffffffffffffffffffffffffffffffffffffff168173ffffffffffffffffffffffffffffffffffffffff1614151515610a2e57600080fd5b3073ffffffffffffffffffffffffffffffffffffffff168173ffffffffffffffffffffffffffffffffffffffff1614151515610a6957600080fd5b610a7384846113c8565b91505092915050565b6000600360149054906101000a900460ff16151515610a9a57600080fd5b610aa483836115e7565b905092915050565b6000600260008473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002054905092915050565b600360009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff16141515610b8f57600080fd5b600073ffffffffffffffffffffffffffffffffffffffff168173ffffffffffffffffffffffffffffffffffffffff1614151515610bcb57600080fd5b8073ffffffffffffffffffffffffffffffffffffffff16600360009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff167f8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e060405160405180910390a380600360006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555050565b600081600260003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060008573ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020819055508273ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff167f8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925846040518082815260200191505060405180910390a36001905092915050565b60008073ffffffffffffffffffffffffffffffffffffffff168373ffffffffffffffffffffffffffffffffffffffff1614151515610dba57600080fd5b6000808573ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020548211151515610e0757600080fd5b600260008573ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020548211151515610e9257600080fd5b610ee3826000808773ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020546117e390919063ffffffff16565b6000808673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002081905550610f76826000808673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020546117fc90919063ffffffff16565b6000808573ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000208190555061104782600260008773ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020546117e390919063ffffffff16565b600260008673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020819055508273ffffffffffffffffffffffffffffffffffffffff168473ffffffffffffffffffffffffffffffffffffffff167fddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef846040518082815260200191505060405180910390a3600190509392505050565b600080600260003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060008573ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002054905080831115611248576000600260003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060008673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020819055506112dc565b61125b83826117e390919063ffffffff16565b600260003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060008673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020819055505b8373ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff167f8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925600260003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060008873ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020546040518082815260200191505060405180910390a3600191505092915050565b60008073ffffffffffffffffffffffffffffffffffffffff168373ffffffffffffffffffffffffffffffffffffffff161415151561140557600080fd5b6000803373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002054821115151561145257600080fd5b6114a3826000803373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020546117e390919063ffffffff16565b6000803373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002081905550611536826000808673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020546117fc90919063ffffffff16565b6000808573ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020819055508273ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff167fddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef846040518082815260200191505060405180910390a36001905092915050565b600061167882600260003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060008673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020546117fc90919063ffffffff16565b600260003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060008573ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020819055508273ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff167f8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925600260003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060008773ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020546040518082815260200191505060405180910390a36001905092915050565b60008282111515156117f157fe5b818303905092915050565b6000818301905082811015151561180f57fe5b809050929150505600a165627a7a72305820ffa710f4c82e1f12645713d71da89f0c795cce49fbe12e060ea17f520d6413f800290000000000000000000000000000000000000000204fce5e3e25026110000000")
	execution, err := action.NewExecution(
		ta.Addrinfo["producer"].RawAddress, action.EmptyAddress, 1, big.NewInt(0), uint64(10000000), big.NewInt(10), data)
	require.NoError(err)
	execution, err = execution.Sign(ta.Addrinfo["producer"])
	require.NoError(err)
	blk, err := bc.MintNewBlock(nil, nil, []*action.Execution{execution}, ta.Addrinfo["producer"], "")
	require.NoError(err)
	require.Nil(bc.CommitBlock(blk))
	require.Equal(1, len(blk.receipts))

	eHash := execution.Hash()
	r, _ := bc.GetReceiptByExecutionHash(eHash)
	require.Equal(eHash, r.Hash)
	contract := r.ContractAddress
	h, _ := iotxaddress.GetPubkeyHash(contract)
	contractAddrHash := byteutil.BytesTo20B(h)
	code, err := bc.GetFactory().GetCode(contractAddrHash)
	require.Nil(err)
	require.Equal(data[335:len(data)-32], code)

	logger.Warn().Msg("======= Transfer to alfa")
	data, _ = hex.DecodeString("a9059cbb")
	alfa := hash.ZeroHash32B
	to, _ := iotxaddress.GetPubkeyHash(ta.Addrinfo["alfa"].RawAddress)
	alfa.SetBytes(to)
	value := hash.ZeroHash32B
	// send 10000 token to Alfa
	h = value[24:]
	binary.BigEndian.PutUint64(h, 10000)
	data = append(data, alfa[:]...)
	data = append(data, value[:]...)
	logger.Warn().Hex("v", data[:]).Msg("TestER")
	execution, err = action.NewExecution(
		ta.Addrinfo["producer"].RawAddress, contract, 2, big.NewInt(0), uint64(10000000), big.NewInt(10), data)
	require.NoError(err)
	execution, err = execution.Sign(ta.Addrinfo["producer"])
	require.NoError(err)
	// send 20000 token to bravo
	data, _ = hex.DecodeString("a9059cbb")
	bravo := hash.ZeroHash32B
	to, _ = iotxaddress.GetPubkeyHash(ta.Addrinfo["bravo"].RawAddress)
	bravo.SetBytes(to)
	value = hash.ZeroHash32B
	h = value[24:]
	binary.BigEndian.PutUint64(h, 20000)
	data = append(data, bravo[:]...)
	data = append(data, value[:]...)
	logger.Warn().Hex("v", data[:]).Msg("TestER")
	ex2, err := action.NewExecution(
		ta.Addrinfo["producer"].RawAddress, contract, 3, big.NewInt(0), uint64(10000000), big.NewInt(10), data)
	require.NoError(err)
	ex2, err = ex2.Sign(ta.Addrinfo["producer"])
	require.NoError(err)
	blk, err = bc.MintNewBlock(nil, nil, []*action.Execution{execution, ex2}, ta.Addrinfo["producer"], "")
	require.NoError(err)
	require.Nil(bc.CommitBlock(blk))

	logger.Warn().Msg("======= Transfer to bravo")
	// alfa send 2000 to bravo
	data, _ = hex.DecodeString("a9059cbb")
	value = hash.ZeroHash32B
	h = value[24:]
	binary.BigEndian.PutUint64(h, 2000)
	data = append(data, bravo[:]...)
	data = append(data, value[:]...)
	logger.Warn().Hex("v", data[:]).Msg("TestER")
	ex3, err := action.NewExecution(
		ta.Addrinfo["alfa"].RawAddress, contract, 1, big.NewInt(0), uint64(10000000), big.NewInt(10), data)
	require.NoError(err)
	ex3, err = ex3.Sign(ta.Addrinfo["alfa"])
	require.NoError(err)
	blk, err = bc.MintNewBlock(nil, nil, []*action.Execution{ex3}, ta.Addrinfo["alfa"], "")
	require.NoError(err)
	require.Nil(bc.CommitBlock(blk))

	// get balance
	data, _ = hex.DecodeString("70a08231")
	data = append(data, alfa[:]...)
	logger.Warn().Hex("v", data[:]).Msg("TestER")
	execution, err = action.NewExecution(
		ta.Addrinfo["producer"].RawAddress, contract, 4, big.NewInt(0), uint64(10000000), big.NewInt(10), data)
	require.NoError(err)
	execution, err = execution.Sign(ta.Addrinfo["producer"])
	require.NoError(err)
	blk, err = bc.MintNewBlock(nil, nil, []*action.Execution{execution}, ta.Addrinfo["producer"], "")
	require.NoError(err)
	require.Nil(bc.CommitBlock(blk))

	// verify balance
	eHash = execution.Hash()
	r, _ = bc.GetReceiptByExecutionHash(eHash)
	require.Equal(eHash, r.Hash)
	h = r.ReturnValue[len(r.ReturnValue)-8:]
	amount := binary.BigEndian.Uint64(h)
	require.Equal(uint64(10000), amount)
}
