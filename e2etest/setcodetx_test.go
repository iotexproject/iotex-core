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

func TestEstimateGas(t *testing.T) {
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
