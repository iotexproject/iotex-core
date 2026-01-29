package e2etest

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/config"
	"github.com/iotexproject/iotex-core/v2/pkg/unit"
	"github.com/iotexproject/iotex-core/v2/pkg/util/assertions"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil"
)

func TestSetCodeTx_E2E(t *testing.T) {
	r := require.New(t)
	sender := identityset.Address(10).String()
	senderSK := identityset.PrivateKey(10)
	cfg := initCfg(r)
	cfg.API.GRPCPort = testutil.RandomPort()
	cfg.API.HTTPPort = testutil.RandomPort()
	cfg.API.WebSocketPort = 0
	cfg.Plugins[config.GatewayPlugin] = true
	cfg.Chain.EnableAsyncIndexWrite = false
	cfg.Genesis.XinguBetaBlockHeight = 1
	cfg.Genesis.ToBeEnabledBlockHeight = 5 // enable setcode tx
	cfg.Genesis.InitBalanceMap[sender] = unit.ConvertIotxToRau(1000000).String()
	testutil.NormalizeGenesisHeights(&cfg.Genesis.Blockchain)
	test := newE2ETest(t, cfg)
	chainID := cfg.Chain.ID
	evmNetworkID := cfg.Chain.EVMNetworkID
	gasPrice := big.NewInt(unit.Qev)
	gasFeeCap := big.NewInt(unit.Qev * 2)
	gasTipCap := big.NewInt(1)

	genTransfers := func(n int) []*actionWithTime {
		newLegacyTx := func(nonce uint64) action.TxCommonInternal {
			return action.NewLegacyTx(chainID, nonce, gasLimit, gasPrice)
		}
		txs := make([]*actionWithTime, n)
		to := identityset.Address(11).String()
		value := big.NewInt(1)
		for i := 0; i < n; i++ {
			transfer := action.NewTransfer(value, to, nil)
			etx, err := action.Sign(action.NewEnvelope(newLegacyTx(test.nonceMgr.pop(sender)), transfer), senderSK)
			r.NoError(err)
			txs[i] = &actionWithTime{etx, time.Now()}
		}
		return txs
	}
	newSetCodeTxWeb3 := func(nonce uint64, sk crypto.PrivateKey, addr string, value *big.Int, data []byte, auths []types.SetCodeAuthorization) (*action.SealedEnvelope, error) {
		var to common.Address
		if addr != "" {
			to = common.BytesToAddress(mustNoErr(address.FromString(addr)).Bytes())
		}
		txdata := types.SetCodeTx{
			ChainID:   uint256.NewInt(uint64(evmNetworkID)),
			Nonce:     nonce,
			GasTipCap: uint256.MustFromBig(gasTipCap),
			GasFeeCap: uint256.MustFromBig(gasFeeCap),
			Gas:       gasLimit,
			To:        to,
			Value:     uint256.MustFromBig(value),
			Data:      data,
			AuthList:  auths,
		}
		tx := types.MustSignNewTx(sk.EcdsaPrivateKey().(*ecdsa.PrivateKey), types.LatestSignerForChainID(txdata.ChainID.ToBig()), &txdata)
		_, sig, pubkey, err := action.ExtractTypeSigPubkey(tx)
		if err != nil {
			return nil, err
		}
		req := &iotextypes.Action{
			Core:         mustNoErr(action.EthRawToContainer(chainID, hex.EncodeToString(mustNoErr(tx.MarshalBinary())))),
			SenderPubKey: pubkey.Bytes(),
			Signature:    sig,
			Encoding:     iotextypes.Encoding_TX_CONTAINER,
		}
		return (&action.Deserializer{}).SetEvmNetworkID(evmNetworkID).ActionToSealedEnvelope(req)
	}

	newDynamicTxWeb3 := func(nonce uint64, sk crypto.PrivateKey, addr string, value *big.Int, data []byte, acl types.AccessList) (*action.SealedEnvelope, error) {
		var to *common.Address
		if addr != "" {
			tmp := common.BytesToAddress(mustNoErr(address.FromString(addr)).Bytes())
			to = &tmp
		}
		txdata := types.DynamicFeeTx{
			ChainID:    uint256.NewInt(uint64(evmNetworkID)).ToBig(),
			Nonce:      nonce,
			GasTipCap:  uint256.MustFromBig(gasTipCap).ToBig(),
			GasFeeCap:  uint256.MustFromBig(gasFeeCap).ToBig(),
			Gas:        gasLimit,
			To:         to,
			Value:      uint256.MustFromBig(value).ToBig(),
			Data:       data,
			AccessList: acl,
		}
		tx := types.MustSignNewTx(sk.EcdsaPrivateKey().(*ecdsa.PrivateKey), types.LatestSignerForChainID(txdata.ChainID), &txdata)
		_, sig, pubkey, err := action.ExtractTypeSigPubkey(tx)
		if err != nil {
			return nil, err
		}
		req := &iotextypes.Action{
			Core:         mustNoErr(action.EthRawToContainer(chainID, hex.EncodeToString(mustNoErr(tx.MarshalBinary())))),
			SenderPubKey: pubkey.Bytes(),
			Signature:    sig,
			Encoding:     iotextypes.Encoding_TX_CONTAINER,
		}
		return (&action.Deserializer{}).SetEvmNetworkID(evmNetworkID).ActionToSealedEnvelope(req)
	}

	contractV2Address := stakingContractV2Address
	contractV2AddressEth := common.BytesToAddress(assertions.MustNoErrorV(address.FromString(contractV2Address)).Bytes())
	contractV3Address := "io1dkqh5mu9djfas3xyrmzdv9frsmmytel4mp7a64"
	contractV3AddressEth := common.BytesToAddress(assertions.MustNoErrorV(address.FromString(contractV3Address)).Bytes())
	bytecodeV3, err := hex.DecodeString(stakingContractV3Bytecode)
	r.NoError(err)
	mustCallDataV3 := func(m string, args ...any) []byte {
		data, err := abiCall(stakingContractV3ABI, m, args...)
		r.NoError(err)
		return data
	}
	var (
		contractCreator = 1
		beneficiaryID   = 10
		minAmount       = unit.ConvertIotxToRau(1000)
	)
	test.run([]*testcase{
		{
			name: "reject setcodetx before prague",
			act: &actionWithTime{
				mustNoErr(newSetCodeTxWeb3(test.nonceMgr[sender], senderSK, sender, big.NewInt(1), nil, nil)),
				time.Now(),
			},
			expect: []actionExpect{&functionExpect{func(test *e2etest, act *action.SealedEnvelope, receipt *action.Receipt, err error) {
				r.ErrorIs(err, action.ErrInvalidAct, "setcode tx should be rejected before Prague hard fork")
			}}},
		},
		{
			name: "deploy_contract_v3",
			acts: []*actionWithTime{
				{mustNoErr(action.SignedExecution("", identityset.PrivateKey(contractCreator), test.nonceMgr.pop(identityset.Address(contractCreator).String()), big.NewInt(0), gasLimit, gasPrice, append(bytecodeV3, mustCallDataV3("", minAmount, contractV2AddressEth)...), action.WithChainID(chainID))), time.Now()},
				{mustNoErr(action.SignedExecution(contractV3Address, identityset.PrivateKey(contractCreator), test.nonceMgr.pop(identityset.Address(contractCreator).String()), big.NewInt(0), gasLimit, gasPrice, mustCallDataV3("setBeneficiary(address)", common.BytesToAddress(identityset.Address(beneficiaryID).Bytes())), action.WithChainID(chainID))), time.Now()},
			},
			blockExpect: func(test *e2etest, blk *block.Block, err error) {
				r.NoError(err)
				r.EqualValues(3, len(blk.Receipts))
				for _, receipt := range blk.Receipts {
					r.Equal(uint64(iotextypes.ReceiptStatus_Success), receipt.Status)
				}
				r.Equal(contractV3Address, blk.Receipts[0].ContractAddress)
			},
		},
	})
	var (
		skipBlocks           = cfg.Genesis.ToBeEnabledBlockHeight - test.cs.Blockchain().TipHeight()
		accountContract      = 7
		candOwnerID          = 3
		stakeAmount          = unit.ConvertIotxToRau(10000)
		secondsPerDay        = 24 * 3600
		stakeDurationSeconds = big.NewInt(int64(secondsPerDay)) // 1 day
	)
	if skipBlocks < 0 {
		skipBlocks = 0
	}
	auth, err := types.SignSetCode(identityset.PrivateKey(accountContract).EcdsaPrivateKey().(*ecdsa.PrivateKey), types.SetCodeAuthorization{
		ChainID: *uint256.NewInt(uint64(evmNetworkID)),
		Address: contractV3AddressEth,
		Nonce:   test.nonceMgr.pop(identityset.Address(accountContract).String()),
	})
	r.NoError(err)
	auths := []types.SetCodeAuthorization{auth}
	calldata := mustCallDataV3("stake(uint256,address)", stakeDurationSeconds, common.BytesToAddress(identityset.Address(candOwnerID).Bytes()))
	test.run([]*testcase{
		{
			name:    "empty auth list",
			preActs: genTransfers(int(skipBlocks)),
			act: &actionWithTime{
				mustNoErr(newSetCodeTxWeb3(test.nonceMgr[sender], senderSK, sender, big.NewInt(1), nil, []types.SetCodeAuthorization{})),
				time.Now(),
			},
			expect: []actionExpect{&functionExpect{func(test *e2etest, act *action.SealedEnvelope, receipt *action.Receipt, err error) {
				r.ErrorIs(err, action.ErrEmptyAuthList, "setcode tx with empty auth list should be rejected")
			}}},
		},
		{
			name: "setcode and stake",
			preFunc: func(*e2etest) {
				code, err := test.ethcli.CodeAt(context.Background(), common.Address(identityset.Address(accountContract).Bytes()), nil)
				r.NoError(err)
				delegate, isDelegated := types.ParseDelegation(code)
				r.False(isDelegated)
				r.Equal(common.Address{}, delegate)
			},
			act: &actionWithTime{
				mustNoErr(newSetCodeTxWeb3(test.nonceMgr.pop(sender), senderSK, identityset.Address(accountContract).String(), stakeAmount, calldata, auths)),
				time.Now(),
			},
			blockExpect: func(test *e2etest, blk *block.Block, err error) {
				r.NoError(err)
				r.EqualValues(2, len(blk.Receipts))
				r.Equal(uint64(iotextypes.ReceiptStatus_Success), blk.Receipts[0].Status)
				// verify code is set correctly
				code, err := test.ethcli.CodeAt(context.Background(), common.Address(identityset.Address(accountContract).Bytes()), nil)
				r.NoError(err)
				delegate, isDelegated := types.ParseDelegation(code)
				r.True(isDelegated, "contract code not set correctly by setcodetx")
				r.Equal(contractV3AddressEth, delegate, "contract code not set correctly by setcodetx")
				// verify stake is processed correctly
				btkIdxs, err := parseV3StakedBucketIdx(identityset.Address(accountContract).String(), blk.Receipts[0])
				r.NoError(err)
				r.Equal(1, len(btkIdxs))
			},
		},
	})

	auth, err = types.SignSetCode(identityset.PrivateKey(accountContract).EcdsaPrivateKey().(*ecdsa.PrivateKey), types.SetCodeAuthorization{
		ChainID: *uint256.NewInt(uint64(evmNetworkID)),
		Address: common.Address{},
		Nonce:   test.nonceMgr.pop(identityset.Address(accountContract).String()),
	})
	r.NoError(err)
	resetAuths := []types.SetCodeAuthorization{auth}
	test.run([]*testcase{
		{
			name: "reset code",
			act: &actionWithTime{
				mustNoErr(newSetCodeTxWeb3(test.nonceMgr.pop(sender), senderSK, identityset.Address(accountContract).String(), big.NewInt(0), nil, resetAuths)),
				time.Now(),
			},
			blockExpect: func(test *e2etest, blk *block.Block, err error) {
				r.NoError(err)
				r.EqualValues(2, len(blk.Receipts))
				r.Equal(uint64(iotextypes.ReceiptStatus_Success), blk.Receipts[0].Status)
				// verify code is reset correctly
				code, err := test.ethcli.CodeAt(context.Background(), common.Address(identityset.Address(accountContract).Bytes()), nil)
				r.NoError(err)
				r.Equal(0, len(code), "contract code not reset correctly by setcodetx")
			},
		},
	})

	// setcodetx gas cost test
	accountContract2 := 8
	auth, err = types.SignSetCode(identityset.PrivateKey(accountContract2).EcdsaPrivateKey().(*ecdsa.PrivateKey), types.SetCodeAuthorization{
		ChainID: *uint256.NewInt(uint64(evmNetworkID)),
		Address: contractV3AddressEth,
		Nonce:   test.nonceMgr.pop(identityset.Address(accountContract2).String()),
	})
	r.NoError(err)
	auths = []types.SetCodeAuthorization{auth}
	test.run([]*testcase{
		{
			name: "setcodetx gas cost",
			acts: []*actionWithTime{
				{mustNoErr(newDynamicTxWeb3(test.nonceMgr.pop(sender), senderSK, contractV3Address, stakeAmount, calldata, types.AccessList{{Address: contractV3AddressEth}})), time.Now()},
				{mustNoErr(newSetCodeTxWeb3(test.nonceMgr.pop(sender), senderSK, identityset.Address(accountContract2).String(), stakeAmount, calldata, auths)), time.Now()},
			},
			blockExpect: func(test *e2etest, blk *block.Block, err error) {
				r.NoError(err)
				r.EqualValues(3, len(blk.Receipts))
				r.Equal(uint64(iotextypes.ReceiptStatus_Success), blk.Receipts[0].Status)
				r.Equal(uint64(iotextypes.ReceiptStatus_Success), blk.Receipts[1].Status)
				gasUsedDynamic := blk.Receipts[0].GasConsumed
				gasUsedSetCode := blk.Receipts[1].GasConsumed
				t.Logf("dynamic fee tx gas used: %d, setcodetx gas used: %d", gasUsedDynamic, gasUsedSetCode)
				r.Less(gasUsedSetCode, gasUsedDynamic*11/10, "setcodetx gas used should be less than 10% more than dynamic fee tx")
			},
		},
	})
}

// TestSetCodeTx_BatchTransfer tests the batch transfer scenario using EIP-7702
// This demonstrates one of the key use cases of EIP-7702: executing multiple transfers in a single transaction
func TestSetCodeTx_BatchTransfer(t *testing.T) {
	r := require.New(t)
	sender := identityset.Address(10).String()
	senderSK := identityset.PrivateKey(10)
	cfg := initCfg(r)
	cfg.API.GRPCPort = testutil.RandomPort()
	cfg.API.HTTPPort = testutil.RandomPort()
	cfg.API.WebSocketPort = 0
	cfg.Plugins[config.GatewayPlugin] = true
	cfg.Chain.EnableAsyncIndexWrite = false
	cfg.Genesis.XinguBetaBlockHeight = 1
	cfg.Genesis.ToBeEnabledBlockHeight = 1 // enable setcode tx from the start
	cfg.Genesis.InitBalanceMap[sender] = unit.ConvertIotxToRau(1000000).String()
	testutil.NormalizeGenesisHeights(&cfg.Genesis.Blockchain)
	test := newE2ETest(t, cfg)
	chainID := cfg.Chain.ID
	evmNetworkID := cfg.Chain.EVMNetworkID
	gasPrice := big.NewInt(unit.Qev)
	gasFeeCap := big.NewInt(unit.Qev * 2)
	gasTipCap := big.NewInt(1)

	// Deploy BatchTransfer contract
	batchTransferContractAddress := "io1dkqh5mu9djfas3xyrmzdv9frsmmytel4mp7a64"
	batchTransferContractAddressEth := common.BytesToAddress(assertions.MustNoErrorV(address.FromString(batchTransferContractAddress)).Bytes())
	bytecode, err := hex.DecodeString(batchTransferBytecode)
	r.NoError(err)

	mustBatchTransferCalldata := func(recipients []common.Address, amounts []*big.Int) []byte {
		data, err := abiCall(batchTransferContractABI, "batchTransfer(address[],uint256[])", recipients, amounts)
		r.NoError(err)
		return data
	}

	newSetCodeTxWeb3 := func(nonce uint64, sk crypto.PrivateKey, addr string, value *big.Int, data []byte, auths []types.SetCodeAuthorization) (*action.SealedEnvelope, error) {
		var to common.Address
		if addr != "" {
			to = common.BytesToAddress(mustNoErr(address.FromString(addr)).Bytes())
		}
		txdata := types.SetCodeTx{
			ChainID:   uint256.NewInt(uint64(evmNetworkID)),
			Nonce:     nonce,
			GasTipCap: uint256.MustFromBig(gasTipCap),
			GasFeeCap: uint256.MustFromBig(gasFeeCap),
			Gas:       gasLimit,
			To:        to,
			Value:     uint256.MustFromBig(value),
			Data:      data,
			AuthList:  auths,
		}
		tx := types.MustSignNewTx(sk.EcdsaPrivateKey().(*ecdsa.PrivateKey), types.LatestSignerForChainID(txdata.ChainID.ToBig()), &txdata)
		_, sig, pubkey, err := action.ExtractTypeSigPubkey(tx)
		if err != nil {
			return nil, err
		}
		req := &iotextypes.Action{
			Core:         mustNoErr(action.EthRawToContainer(chainID, hex.EncodeToString(mustNoErr(tx.MarshalBinary())))),
			SenderPubKey: pubkey.Bytes(),
			Signature:    sig,
			Encoding:     iotextypes.Encoding_TX_CONTAINER,
		}
		return (&action.Deserializer{}).SetEvmNetworkID(evmNetworkID).ActionToSealedEnvelope(req)
	}

	// Recipients for batch transfer test
	recipient1 := identityset.Address(20)
	recipient2 := identityset.Address(21)
	recipient3 := identityset.Address(22)
	recipients := []common.Address{
		common.BytesToAddress(recipient1.Bytes()),
		common.BytesToAddress(recipient2.Bytes()),
		common.BytesToAddress(recipient3.Bytes()),
	}
	transferAmount := unit.ConvertIotxToRau(100)
	amounts := []*big.Int{transferAmount, transferAmount, transferAmount}
	totalAmount := new(big.Int).Mul(transferAmount, big.NewInt(int64(len(amounts))))

	var (
		contractCreator = 1
		accountContract = 11 // EOA that will delegate to batch transfer contract
	)

	test.run([]*testcase{
		{
			name: "deploy_batch_transfer_contract",
			acts: []*actionWithTime{
				{mustNoErr(action.SignedExecution("", identityset.PrivateKey(contractCreator), test.nonceMgr.pop(identityset.Address(contractCreator).String()), big.NewInt(0), gasLimit, gasPrice, bytecode, action.WithChainID(chainID))), time.Now()},
			},
			blockExpect: func(test *e2etest, blk *block.Block, err error) {
				r.NoError(err)
				r.EqualValues(2, len(blk.Receipts))
				r.Equal(uint64(iotextypes.ReceiptStatus_Success), blk.Receipts[0].Status)
				r.Equal(batchTransferContractAddress, blk.Receipts[0].ContractAddress)
			},
		},
	})

	// Get initial balances
	var initialBalances [3]*big.Int
	for i, addr := range []address.Address{recipient1, recipient2, recipient3} {
		balance, err := test.ethcli.BalanceAt(context.Background(), common.BytesToAddress(addr.Bytes()), nil)
		r.NoError(err)
		initialBalances[i] = balance
	}

	// Create authorization for account to delegate to batch transfer contract
	auth, err := types.SignSetCode(identityset.PrivateKey(accountContract).EcdsaPrivateKey().(*ecdsa.PrivateKey), types.SetCodeAuthorization{
		ChainID: *uint256.NewInt(uint64(evmNetworkID)),
		Address: batchTransferContractAddressEth,
		Nonce:   test.nonceMgr.pop(identityset.Address(accountContract).String()),
	})
	r.NoError(err)
	auths := []types.SetCodeAuthorization{auth}

	// Build batch transfer calldata
	calldata := mustBatchTransferCalldata(recipients, amounts)

	test.run([]*testcase{
		{
			name: "setcode_and_batch_transfer",
			preFunc: func(*e2etest) {
				// Verify account has no code before setcode
				code, err := test.ethcli.CodeAt(context.Background(), common.Address(identityset.Address(accountContract).Bytes()), nil)
				r.NoError(err)
				r.Equal(0, len(code), "account should have no code before setcode")
			},
			act: &actionWithTime{
				mustNoErr(newSetCodeTxWeb3(test.nonceMgr.pop(sender), senderSK, identityset.Address(accountContract).String(), totalAmount, calldata, auths)),
				time.Now(),
			},
			blockExpect: func(test *e2etest, blk *block.Block, err error) {
				r.NoError(err)
				r.EqualValues(2, len(blk.Receipts))
				r.Equal(uint64(iotextypes.ReceiptStatus_Success), blk.Receipts[0].Status)

				// Verify code delegation is set correctly
				code, err := test.ethcli.CodeAt(context.Background(), common.Address(identityset.Address(accountContract).Bytes()), nil)
				r.NoError(err)
				delegate, isDelegated := types.ParseDelegation(code)
				r.True(isDelegated, "contract code not set correctly by setcodetx")
				r.Equal(batchTransferContractAddressEth, delegate, "delegation target mismatch")

				// Verify all recipients received their transfers
				for i, addr := range []address.Address{recipient1, recipient2, recipient3} {
					balance, err := test.ethcli.BalanceAt(context.Background(), common.BytesToAddress(addr.Bytes()), nil)
					r.NoError(err)
					expectedBalance := new(big.Int).Add(initialBalances[i], transferAmount)
					r.Equal(expectedBalance, balance, "recipient %d balance mismatch: expected %s, got %s", i+1, expectedBalance.String(), balance.String())
				}

				t.Logf("Batch transfer successful: transferred %s wei to %d recipients in a single transaction", transferAmount.String(), len(recipients))
			},
		},
	})
}

// TestSetCodeTx_ERC20ApproveAndTransfer tests EIP-7702 with ERC20 approve + transfer in one transaction
// This demonstrates the classic DEX use case: approve tokens and transfer them atomically
func TestSetCodeTx_ERC20ApproveAndTransfer(t *testing.T) {
	r := require.New(t)
	sender := identityset.Address(10).String()
	senderSK := identityset.PrivateKey(10)
	cfg := initCfg(r)
	cfg.API.GRPCPort = testutil.RandomPort()
	cfg.API.HTTPPort = testutil.RandomPort()
	cfg.API.WebSocketPort = 0
	cfg.Plugins[config.GatewayPlugin] = true
	cfg.Chain.EnableAsyncIndexWrite = false
	cfg.Genesis.XinguBetaBlockHeight = 1
	cfg.Genesis.ToBeEnabledBlockHeight = 1 // enable setcode tx from the start
	cfg.Genesis.InitBalanceMap[sender] = unit.ConvertIotxToRau(1000000).String()
	testutil.NormalizeGenesisHeights(&cfg.Genesis.Blockchain)
	test := newE2ETest(t, cfg)
	chainID := cfg.Chain.ID
	evmNetworkID := cfg.Chain.EVMNetworkID
	gasPrice := big.NewInt(unit.Qev)
	gasFeeCap := big.NewInt(unit.Qev * 2)
	gasTipCap := big.NewInt(1)

	// Deploy contracts
	erc20Bytecode, err := hex.DecodeString(erc20Bytecode)
	r.NoError(err)
	multicallBytecode, err := hex.DecodeString(multicallBytecode)
	r.NoError(err)

	var erc20ContractAddress string
	var erc20ContractAddressEth common.Address
	var multicallContractAddress string
	var multicallContractAddressEth common.Address

	mustERC20Calldata := func(m string, args ...any) []byte {
		data, err := abiCall(erc20ABI, m, args...)
		r.NoError(err)
		return data
	}

	// Multicall struct for encoding calls
	type Call struct {
		Target common.Address
		Data   []byte
	}

	mustMulticallCalldata := func(calls []Call) []byte {
		data, err := abiCall(multicallContractABI, "multicall((address,bytes)[])", calls)
		r.NoError(err)
		return data
	}

	newSetCodeTxWeb3 := func(nonce uint64, sk crypto.PrivateKey, addr string, value *big.Int, data []byte, auths []types.SetCodeAuthorization) (*action.SealedEnvelope, error) {
		var to common.Address
		if addr != "" {
			to = common.BytesToAddress(mustNoErr(address.FromString(addr)).Bytes())
		}
		txdata := types.SetCodeTx{
			ChainID:   uint256.NewInt(uint64(evmNetworkID)),
			Nonce:     nonce,
			GasTipCap: uint256.MustFromBig(gasTipCap),
			GasFeeCap: uint256.MustFromBig(gasFeeCap),
			Gas:       gasLimit,
			To:        to,
			Value:     uint256.MustFromBig(value),
			Data:      data,
			AuthList:  auths,
		}
		tx := types.MustSignNewTx(sk.EcdsaPrivateKey().(*ecdsa.PrivateKey), types.LatestSignerForChainID(txdata.ChainID.ToBig()), &txdata)
		_, sig, pubkey, err := action.ExtractTypeSigPubkey(tx)
		if err != nil {
			return nil, err
		}
		req := &iotextypes.Action{
			Core:         mustNoErr(action.EthRawToContainer(chainID, hex.EncodeToString(mustNoErr(tx.MarshalBinary())))),
			SenderPubKey: pubkey.Bytes(),
			Signature:    sig,
			Encoding:     iotextypes.Encoding_TX_CONTAINER,
		}
		return (&action.Deserializer{}).SetEvmNetworkID(evmNetworkID).ActionToSealedEnvelope(req)
	}

	var (
		contractCreator = 1
		tokenHolder     = 12 // EOA that holds tokens and will delegate to multicall
		recipient       = identityset.Address(23)
	)
	tokenHolderAddr := identityset.Address(tokenHolder)
	tokenHolderAddrEth := common.BytesToAddress(tokenHolderAddr.Bytes())
	recipientEth := common.BytesToAddress(recipient.Bytes())
	tokenAmount := big.NewInt(1000000) // 1M tokens

	// Step 1: Deploy ERC20 and Multicall contracts
	test.run([]*testcase{
		{
			name: "deploy_erc20_and_multicall",
			acts: []*actionWithTime{
				{mustNoErr(action.SignedExecution("", identityset.PrivateKey(contractCreator), test.nonceMgr.pop(identityset.Address(contractCreator).String()), big.NewInt(0), gasLimit, gasPrice, erc20Bytecode, action.WithChainID(chainID))), time.Now()},
				{mustNoErr(action.SignedExecution("", identityset.PrivateKey(contractCreator), test.nonceMgr.pop(identityset.Address(contractCreator).String()), big.NewInt(0), gasLimit, gasPrice, multicallBytecode, action.WithChainID(chainID))), time.Now()},
			},
			blockExpect: func(test *e2etest, blk *block.Block, err error) {
				r.NoError(err)
				r.EqualValues(3, len(blk.Receipts))
				r.Equal(uint64(iotextypes.ReceiptStatus_Success), blk.Receipts[0].Status)
				r.Equal(uint64(iotextypes.ReceiptStatus_Success), blk.Receipts[1].Status)
				erc20ContractAddress = blk.Receipts[0].ContractAddress
				erc20ContractAddressEth = common.BytesToAddress(assertions.MustNoErrorV(address.FromString(erc20ContractAddress)).Bytes())
				multicallContractAddress = blk.Receipts[1].ContractAddress
				multicallContractAddressEth = common.BytesToAddress(assertions.MustNoErrorV(address.FromString(multicallContractAddress)).Bytes())
				t.Logf("ERC20 deployed at: %s, Multicall deployed at: %s", erc20ContractAddress, multicallContractAddress)
			},
		},
	})

	// Step 2: Mint tokens to token holder
	test.run([]*testcase{
		{
			name: "mint_tokens_to_holder",
			acts: []*actionWithTime{
				{mustNoErr(action.SignedExecution(erc20ContractAddress, identityset.PrivateKey(contractCreator), test.nonceMgr.pop(identityset.Address(contractCreator).String()), big.NewInt(0), gasLimit, gasPrice, mustERC20Calldata("mint(address,uint256)", tokenHolderAddrEth, tokenAmount), action.WithChainID(chainID))), time.Now()},
			},
			blockExpect: func(test *e2etest, blk *block.Block, err error) {
				r.NoError(err)
				r.EqualValues(2, len(blk.Receipts))
				r.Equal(uint64(iotextypes.ReceiptStatus_Success), blk.Receipts[0].Status)
				t.Logf("Minted %s tokens to %s", tokenAmount.String(), tokenHolderAddr.String())
			},
		},
	})

	// Step 3: Token holder delegates to Multicall and executes approve + transfer atomically
	auth, err := types.SignSetCode(identityset.PrivateKey(tokenHolder).EcdsaPrivateKey().(*ecdsa.PrivateKey), types.SetCodeAuthorization{
		ChainID: *uint256.NewInt(uint64(evmNetworkID)),
		Address: multicallContractAddressEth,
		Nonce:   test.nonceMgr.pop(tokenHolderAddr.String()),
	})
	r.NoError(err)
	auths := []types.SetCodeAuthorization{auth}

	// Create multicall data: approve spender (sender) + transfer to recipient
	// Note: Since we're executing in the context of tokenHolder (via delegation),
	// we can directly call transfer instead of transferFrom
	transferAmount := big.NewInt(500000)
	calls := []Call{
		{
			Target: erc20ContractAddressEth,
			Data:   mustERC20Calldata("approve(address,uint256)", common.BytesToAddress(identityset.Address(10).Bytes()), transferAmount),
		},
		{
			Target: erc20ContractAddressEth,
			Data:   mustERC20Calldata("transfer(address,uint256)", recipientEth, transferAmount),
		},
	}
	multicallData := mustMulticallCalldata(calls)

	test.run([]*testcase{
		{
			name: "setcode_multicall_approve_and_transfer",
			preFunc: func(*e2etest) {
				// Verify token holder has tokens
				balanceData := mustERC20Calldata("balanceOf(address)", tokenHolderAddrEth)
				result, err := test.ethcli.CallContract(context.Background(), ethereum.CallMsg{
					To:   &erc20ContractAddressEth,
					Data: balanceData,
				}, nil)
				r.NoError(err)
				balance := new(big.Int).SetBytes(result)
				r.Equal(tokenAmount, balance, "token holder should have tokens before transfer")

				// Verify recipient has no tokens
				balanceData = mustERC20Calldata("balanceOf(address)", recipientEth)
				result, err = test.ethcli.CallContract(context.Background(), ethereum.CallMsg{
					To:   &erc20ContractAddressEth,
					Data: balanceData,
				}, nil)
				r.NoError(err)
				balance = new(big.Int).SetBytes(result)
				r.True(balance.Sign() == 0, "recipient should have no tokens before transfer")
			},
			act: &actionWithTime{
				mustNoErr(newSetCodeTxWeb3(test.nonceMgr.pop(sender), senderSK, tokenHolderAddr.String(), big.NewInt(0), multicallData, auths)),
				time.Now(),
			},
			blockExpect: func(test *e2etest, blk *block.Block, err error) {
				r.NoError(err)
				r.EqualValues(2, len(blk.Receipts))
				r.Equal(uint64(iotextypes.ReceiptStatus_Success), blk.Receipts[0].Status)

				// Verify code delegation is set correctly
				code, err := test.ethcli.CodeAt(context.Background(), tokenHolderAddrEth, nil)
				r.NoError(err)
				delegate, isDelegated := types.ParseDelegation(code)
				r.True(isDelegated, "code delegation not set")
				r.Equal(multicallContractAddressEth, delegate, "delegation target mismatch")

				// Verify token balances after transfer
				balanceData := mustERC20Calldata("balanceOf(address)", tokenHolderAddrEth)
				result, err := test.ethcli.CallContract(context.Background(), ethereum.CallMsg{
					To:   &erc20ContractAddressEth,
					Data: balanceData,
				}, nil)
				r.NoError(err)
				holderBalance := new(big.Int).SetBytes(result)
				expectedHolderBalance := new(big.Int).Sub(tokenAmount, transferAmount)
				r.Equal(expectedHolderBalance, holderBalance, "token holder balance mismatch")

				balanceData = mustERC20Calldata("balanceOf(address)", recipientEth)
				result, err = test.ethcli.CallContract(context.Background(), ethereum.CallMsg{
					To:   &erc20ContractAddressEth,
					Data: balanceData,
				}, nil)
				r.NoError(err)
				recipientBalance := new(big.Int).SetBytes(result)
				r.Equal(transferAmount, recipientBalance, "recipient balance mismatch")

				// Verify allowance is set
				allowanceData := mustERC20Calldata("allowance(address,address)", tokenHolderAddrEth, common.BytesToAddress(identityset.Address(10).Bytes()))
				result, err = test.ethcli.CallContract(context.Background(), ethereum.CallMsg{
					To:   &erc20ContractAddressEth,
					Data: allowanceData,
				}, nil)
				r.NoError(err)
				allowance := new(big.Int).SetBytes(result)
				r.Equal(transferAmount, allowance, "allowance not set correctly")

				t.Logf("ERC20 approve + transfer successful: approved %s tokens and transferred %s tokens in one tx", transferAmount.String(), transferAmount.String())
			},
		},
	})
}

func TestEstimateGas(t *testing.T) {
	t.Skip("TODO: fix test")
	r := require.New(t)
	sender := identityset.Address(10).String()
	senderSK := identityset.PrivateKey(10)
	cfg := initCfg(r)
	cfg.API.GRPCPort = testutil.RandomPort()
	cfg.API.HTTPPort = testutil.RandomPort()
	cfg.API.WebSocketPort = 0
	cfg.Plugins[config.GatewayPlugin] = true
	cfg.Chain.EnableAsyncIndexWrite = false
	historyIndexPath, err := os.MkdirTemp("", "historyindex")
	r.NoError(err)
	cfg.Chain.HistoryIndexPath = historyIndexPath
	cfg.Genesis.XinguBetaBlockHeight = 1
	cfg.Genesis.ToBeEnabledBlockHeight = 5 // enable setcode tx
	cfg.Genesis.InitBalanceMap[sender] = unit.ConvertIotxToRau(1000000).String()
	testutil.NormalizeGenesisHeights(&cfg.Genesis.Blockchain)
	test := newE2ETest(t, cfg)
	chainID := cfg.Chain.ID
	evmNetworkID := cfg.Chain.EVMNetworkID
	gasPrice := big.NewInt(unit.Qev)
	// gasFeeCap := big.NewInt(unit.Qev * 2)
	// gasTipCap := big.NewInt(1)

	contractV2Address := stakingContractV2Address
	contractV2AddressEth := common.BytesToAddress(assertions.MustNoErrorV(address.FromString(contractV2Address)).Bytes())
	contractV3Address := "io1dkqh5mu9djfas3xyrmzdv9frsmmytel4mp7a64"
	contractV3AddressEth := common.BytesToAddress(assertions.MustNoErrorV(address.FromString(contractV3Address)).Bytes())
	bytecodeV3, err := hex.DecodeString(stakingContractV3Bytecode)
	r.NoError(err)
	mustCallDataV3 := func(m string, args ...any) []byte {
		data, err := abiCall(stakingContractV3ABI, m, args...)
		r.NoError(err)
		return data
	}
	genTransfers := func(n int) []*actionWithTime {
		newLegacyTx := func(nonce uint64) action.TxCommonInternal {
			return action.NewLegacyTx(chainID, nonce, gasLimit, gasPrice)
		}
		txs := make([]*actionWithTime, n)
		to := identityset.Address(11).String()
		value := big.NewInt(1)
		for i := 0; i < n; i++ {
			transfer := action.NewTransfer(value, to, nil)
			etx, err := action.Sign(action.NewEnvelope(newLegacyTx(test.nonceMgr.pop(sender)), transfer), senderSK)
			r.NoError(err)
			txs[i] = &actionWithTime{etx, time.Now()}
		}
		return txs
	}
	var (
		contractCreator = 1
		beneficiaryID   = 10
		minAmount       = unit.ConvertIotxToRau(1000)
	)
	test.run([]*testcase{
		{
			name:    "deploy_contract_v3",
			preActs: genTransfers(int(cfg.Genesis.ToBeEnabledBlockHeight)),
			acts: []*actionWithTime{
				{mustNoErr(action.SignedExecution("", identityset.PrivateKey(contractCreator), test.nonceMgr.pop(identityset.Address(contractCreator).String()), big.NewInt(0), 2*gasLimit, gasPrice, append(bytecodeV3, mustCallDataV3("", minAmount, contractV2AddressEth)...), action.WithChainID(chainID))), time.Now()},
				{mustNoErr(action.SignedExecution(contractV3Address, identityset.PrivateKey(contractCreator), test.nonceMgr.pop(identityset.Address(contractCreator).String()), big.NewInt(0), gasLimit, gasPrice, mustCallDataV3("setBeneficiary(address)", common.BytesToAddress(identityset.Address(beneficiaryID).Bytes())), action.WithChainID(chainID))), time.Now()},
			},
			blockExpect: func(test *e2etest, blk *block.Block, err error) {
				r.NoError(err)
				r.EqualValues(3, len(blk.Receipts))
				for _, receipt := range blk.Receipts {
					r.Equal(uint64(iotextypes.ReceiptStatus_Success), receipt.Status)
				}
				r.Equal(contractV3Address, blk.Receipts[0].ContractAddress)
			},
		},
	})

	to1 := common.BytesToAddress(identityset.Address(11).Bytes())
	to2 := common.BytesToAddress(identityset.Address(12).Bytes())
	auth, err := types.SignSetCode(identityset.PrivateKey(11).EcdsaPrivateKey().(*ecdsa.PrivateKey), types.SetCodeAuthorization{
		ChainID: *uint256.NewInt(uint64(evmNetworkID)),
		Address: contractV3AddressEth,
		Nonce:   test.nonceMgr.pop(identityset.Address(11).String()),
	})
	r.NoError(err)
	auth2, err := types.SignSetCode(identityset.PrivateKey(12).EcdsaPrivateKey().(*ecdsa.PrivateKey), types.SetCodeAuthorization{
		ChainID: *uint256.NewInt(uint64(evmNetworkID)),
		Address: contractV3AddressEth,
		Nonce:   test.nonceMgr.pop(identityset.Address(12).String()),
	})
	r.NoError(err)
	var testcases = []struct {
		blockNumber *big.Int
		call        ethereum.CallMsg
		want        uint64
	}{
		// transfer
		{
			call: ethereum.CallMsg{
				From:     common.BytesToAddress(assertions.MustNoErrorV(address.FromString(sender)).Bytes()),
				To:       &to1,
				GasPrice: gasPrice,
				Value:    unit.ConvertIotxToRau(1),
			},
			want: 21000, // minimum gas for transfer
		},
		// transfer to contract
		{
			call: ethereum.CallMsg{
				From:     common.BytesToAddress(assertions.MustNoErrorV(address.FromString(sender)).Bytes()),
				To:       &contractV3AddressEth,
				GasPrice: gasPrice,
				Value:    unit.ConvertIotxToRau(1),
			},
			want: 21000, // minimum gas for execution
		},
		// setcodetx
		{
			call: ethereum.CallMsg{
				From:              common.BytesToAddress(assertions.MustNoErrorV(address.FromString(sender)).Bytes()),
				To:                &to1,
				GasPrice:          gasPrice,
				Value:             unit.ConvertIotxToRau(1),
				AuthorizationList: []types.SetCodeAuthorization{auth},
			},
			want: 35055, // 10000 (base tx) + 25000 (auth list) + 55 (evm ops)
		},
		// setcodetx with new account
		{
			call: ethereum.CallMsg{
				From:              common.BytesToAddress(assertions.MustNoErrorV(address.FromString(sender)).Bytes()),
				To:                &to2,
				GasPrice:          gasPrice,
				Value:             unit.ConvertIotxToRau(1),
				AuthorizationList: []types.SetCodeAuthorization{auth2},
			},
			want: 35055, // 10000 (base tx) + 25000 (auth list) + 55 (evm ops)
		},
		// big calldata before prague
		{
			blockNumber: big.NewInt(int64(cfg.Genesis.ToBeEnabledBlockHeight - 2)),
			call: ethereum.CallMsg{
				From:     common.BytesToAddress(assertions.MustNoErrorV(address.FromString(sender)).Bytes()),
				To:       &contractV2AddressEth,
				GasPrice: gasPrice,
				Value:    unit.ConvertIotxToRau(1),
				Data:     make([]byte, 10240),
			},
			want: 1_034_000, // 10000 (base) + 10240 * 100 (per byte)
		},
		// big calldata after prague
		{
			blockNumber: big.NewInt(int64(cfg.Genesis.ToBeEnabledBlockHeight)),
			call: ethereum.CallMsg{
				From:     common.BytesToAddress(assertions.MustNoErrorV(address.FromString(sender)).Bytes()),
				To:       &contractV2AddressEth,
				GasPrice: gasPrice,
				Value:    unit.ConvertIotxToRau(1),
				Data:     make([]byte, 10240),
			},
			want: 2_570_000, // 10000 (base) + 10240 * 250 (per byte)
		},
	}

	for i, tc := range testcases {
		gas, err := test.ethcli.EstimateGasAtBlock(context.Background(), tc.call, tc.blockNumber)
		r.NoErrorf(err, "test case %d failed", i)
		r.Equalf(tc.want, gas, "test case %d failed", i)
	}
}
