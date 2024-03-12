package e2etest

import (
	"context"
	"encoding/hex"
	"math/big"
	"sync/atomic"
	"testing"
	"time"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/iotex-address/address"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/action/protocol/execution"
	"github.com/iotexproject/iotex-core/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/blockchain/filedao"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/blockindex"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/state/factory"
	"github.com/iotexproject/iotex-core/testutil"
)

type checkContractExpectation struct {
	errorContains    string
	contractAddress  string
	expectPercentage uint64
	expectReceiver   address.Address
	expectIsApproved bool
}
type sgdTest struct {
	name                string
	data                []string
	checkContractExpect checkContractExpectation
}

func TestSGDRegistry(t *testing.T) {
	r := require.New(t)
	ctx := context.Background()
	cfg := config.Default
	cfg.Chain.EnableAsyncIndexWrite = false
	cfg.Genesis.EnableGravityChainVoting = false
	cfg.Genesis.InitBalanceMap[_executor] = "1000000000000000000000000000"
	registry := protocol.NewRegistry()
	acc := account.NewProtocol(rewarding.DepositGas)
	r.NoError(acc.Register(registry))
	rp := rolldpos.NewProtocol(cfg.Genesis.NumCandidateDelegates, cfg.Genesis.NumDelegates, cfg.Genesis.NumSubEpochs)
	r.NoError(rp.Register(registry))
	factoryCfg := factory.GenerateConfig(cfg.Chain, cfg.Genesis)
	sf, err := factory.NewFactory(factoryCfg, db.NewMemKVStore(), factory.RegistryOption(registry))
	r.NoError(err)
	genericValidator := protocol.NewGenericValidator(sf, accountutil.AccountState)
	ap, err := actpool.NewActPool(cfg.Genesis, sf, cfg.ActPool)
	r.NoError(err)
	ap.AddActionEnvelopeValidators(genericValidator)
	store, err := filedao.NewFileDAOInMemForTest()
	r.NoError(err)
	dao := blockdao.NewBlockDAOWithIndexersAndCache(store, []blockdao.BlockIndexer{sf}, cfg.DB.MaxCacheSize)
	bc := blockchain.NewBlockchain(
		cfg.Chain,
		cfg.Genesis,
		dao,
		factory.NewMinter(sf, ap),
		blockchain.BlockValidatorOption(block.NewValidator(
			sf,
			genericValidator,
		)),
	)
	r.NotNil(bc)
	reward := rewarding.NewProtocol(cfg.Genesis.Rewarding)
	r.NoError(reward.Register(registry))

	ep := execution.NewProtocol(dao.GetBlockHash, rewarding.DepositGasWithSGD, nil, fakeGetBlockTime)
	r.NoError(ep.Register(registry))
	r.NoError(bc.Start(ctx))
	ctx = genesis.WithGenesisContext(ctx, cfg.Genesis)
	r.NoError(sf.Start(ctx))
	defer r.NoError(bc.Stop(ctx))

	_execPriKey, _ := crypto.HexStringToPrivateKey(_executorPriKey)
	data, _ := hex.DecodeString("608060405234801561001057600080fd5b5061001a3361001f565b61006f565b600080546001600160a01b038381166001600160a01b0319831681178455604051919092169283917f8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e09190a35050565b6107be8061007e6000396000f3fe608060405234801561001057600080fd5b50600436106100885760003560e01c8063a0ee93181161005b578063a0ee93181461014d578063c375c2ef14610160578063d7e5fbf314610173578063f2fde38b1461018657600080fd5b806307f7aafb1461008d5780630ad1c2fa146100a2578063715018a61461012a5780638da5cb5b14610132575b600080fd5b6100a061009b3660046106fc565b610199565b005b6100ff6100b03660046106fc565b604080518082018252600080825260209182018190526001600160a01b039384168152600182528290208251808401909352549283168252600160a01b90920460ff1615159181019190915290565b6040805182516001600160a01b03168152602092830151151592810192909252015b60405180910390f35b6100a061028f565b6000546040516001600160a01b039091168152602001610121565b6100a061015b3660046106fc565b6102a3565b6100a061016e3660046106fc565b610381565b6100a061018136600461071e565b61041a565b6100a06101943660046106fc565b6105bd565b6101a1610636565b6001600160a01b03808216600090815260016020526040902080549091166101e45760405162461bcd60e51b81526004016101db90610751565b60405180910390fd5b8054600160a01b900460ff161561023d5760405162461bcd60e51b815260206004820152601c60248201527f436f6e747261637420697320616c726561647920617070726f7665640000000060448201526064016101db565b805460ff60a01b1916600160a01b1781556040516001600160a01b03831681527faf42961ad755cade79794d4122cb0afedc32bf55a0c716dd085fbee2afc6ac55906020015b60405180910390a15050565b610297610636565b6102a16000610690565b565b6102ab610636565b6001600160a01b03808216600090815260016020526040902080549091166102e55760405162461bcd60e51b81526004016101db90610751565b8054600160a01b900460ff1661033d5760405162461bcd60e51b815260206004820152601860248201527f436f6e7472616374206973206e6f7420617070726f766564000000000000000060448201526064016101db565b805460ff60a01b191681556040516001600160a01b03831681527f50132537991c16a2d6bbd27114ed077fb8c757768dcbb2c3c5736f1b1ed3cc3e90602001610283565b610389610636565b6001600160a01b03808216600090815260016020526040902080549091166103c35760405162461bcd60e51b81526004016101db90610751565b6001600160a01b03821660008181526001602090815260409182902080546001600160a81b031916905590519182527f8d30d41865a0b811b9545d879520d2dde9f4cc49e4241f486ad9752bc904b5659101610283565b6001600160a01b0382166104705760405162461bcd60e51b815260206004820152601f60248201527f436f6e747261637420616464726573732063616e6e6f74206265207a65726f0060448201526064016101db565b6001600160a01b0381166104c65760405162461bcd60e51b815260206004820181905260248201527f526563697069656e7420616464726573732063616e6e6f74206265207a65726f60448201526064016101db565b6001600160a01b038216600090815260016020526040902054600160a01b900460ff16156105365760405162461bcd60e51b815260206004820152601c60248201527f436f6e747261637420697320616c726561647920617070726f7665640000000060448201526064016101db565b6040805180820182526001600160a01b038381168083526000602080850182815288851680845260018352928790209551865491511515600160a01b026001600160a81b0319909216951694909417939093179093558351928352908201527f768fb430a0d4b201cb764ab221c316dd14d8babf2e4b2348e05964c6565318b69101610283565b6105c5610636565b6001600160a01b03811661062a5760405162461bcd60e51b815260206004820152602660248201527f4f776e61626c653a206e6577206f776e657220697320746865207a65726f206160448201526564647265737360d01b60648201526084016101db565b61063381610690565b50565b6000546001600160a01b031633146102a15760405162461bcd60e51b815260206004820181905260248201527f4f776e61626c653a2063616c6c6572206973206e6f7420746865206f776e657260448201526064016101db565b600080546001600160a01b038381166001600160a01b0319831681178455604051919092169283917f8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e09190a35050565b80356001600160a01b03811681146106f757600080fd5b919050565b60006020828403121561070e57600080fd5b610717826106e0565b9392505050565b6000806040838503121561073157600080fd5b61073a836106e0565b9150610748602084016106e0565b90509250929050565b6020808252601a908201527f436f6e7472616374206973206e6f74207265676973746572656400000000000060408201526060019056fea26469706673582212209c9909f9fd809a16fec29eeab099fda8ddc415db17cc218c5acefb8b82b99b9864736f6c634300080f0033")
	fixedTime := time.Unix(cfg.Genesis.Timestamp, 0)
	nonce := uint64(0)
	exec, err := action.SignedExecution(action.EmptyAddress, _execPriKey, atomic.AddUint64(&nonce, 1), big.NewInt(0), 10000000, big.NewInt(9000000000000), data)
	r.NoError(err)
	deployHash, err := exec.Hash()
	r.NoError(err)
	r.NoError(ap.Add(context.Background(), exec))
	blk, err := bc.MintNewBlock(fixedTime)
	r.NoError(err)
	r.NoError(bc.CommitBlock(blk))
	receipt := blk.Receipts[0]
	r.Equal(receipt.ActionHash, deployHash)
	r.Equal(receipt.ContractAddress, "io1va03q4lcr608dr3nltwm64sfcz05czjuycsqgn")
	height, err := dao.Height()
	r.NoError(err)
	r.Equal(uint64(1), height)

	contractAddress := receipt.ContractAddress

	indexSGDDBPath, err := testutil.PathOfTempFile(_dBPath + "_sgd")
	r.NoError(err)
	kvstore, err := db.CreateKVStore(db.DefaultConfig, indexSGDDBPath)
	r.NoError(err)
	sgdRegistry := blockindex.NewSGDRegistry(contractAddress, 2, kvstore)
	r.NoError(sgdRegistry.Start(ctx))
	defer func() {
		r.NoError(sgdRegistry.Stop(ctx))
		testutil.CleanupPath(indexSGDDBPath)
	}()

	registerAddress, err := address.FromHex("5b38da6a701c568545dcfcb03fcb875f56beddc4")
	r.NoError(err)
	receiverAddress, err := address.FromHex("78731d3ca6b7e34ac0f824c42a7cc18a495cabab")
	r.NoError(err)
	expectPercentage := uint64(30)
	tests := []sgdTest{
		{
			name: "registerContract",
			data: []string{
				"d7e5fbf30000000000000000000000005b38da6a701c568545dcfcb03fcb875f56beddc400000000000000000000000078731d3ca6b7e34ac0f824c42a7cc18a495cabab",
			},
			checkContractExpect: checkContractExpectation{
				contractAddress:  registerAddress.String(),
				expectReceiver:   receiverAddress,
				expectPercentage: expectPercentage,
				expectIsApproved: false,
			},
		},
		{
			name: "approveContract",
			data: []string{
				"07f7aafb0000000000000000000000005b38da6a701c568545dcfcb03fcb875f56beddc4",
			},
			checkContractExpect: checkContractExpectation{
				contractAddress:  registerAddress.String(),
				expectReceiver:   receiverAddress,
				expectPercentage: expectPercentage,
				expectIsApproved: true,
			},
		},
		{
			name: "registerContractAgain",
			data: []string{
				"d7e5fbf30000000000000000000000005b38da6a701c568545dcfcb03fcb875f56beddc400000000000000000000000078731d3ca6b7e34ac0f824c42a7cc18a495cabab",
			},
			checkContractExpect: checkContractExpectation{
				contractAddress:  registerAddress.String(),
				expectReceiver:   receiverAddress,
				expectPercentage: expectPercentage,
				expectIsApproved: true,
			},
		},
		{
			name: "disapproveContract",
			data: []string{
				"a0ee93180000000000000000000000005b38da6a701c568545dcfcb03fcb875f56beddc4",
			},
			checkContractExpect: checkContractExpectation{
				contractAddress:  registerAddress.String(),
				expectReceiver:   receiverAddress,
				expectPercentage: expectPercentage,
				expectIsApproved: false,
			},
		},
		{
			name: "removeContract",
			data: []string{
				"c375c2ef0000000000000000000000005b38da6a701c568545dcfcb03fcb875f56beddc4",
			},
			checkContractExpect: checkContractExpectation{
				contractAddress: registerAddress.String(),
			},
		},
		{
			name: "approveNoneExistContract",
			data: []string{
				"07f7aafb0000000000000000000000005b38da6a701c568545dcfcb03fcb875f56beddc4",
			},
			checkContractExpect: checkContractExpectation{
				contractAddress: registerAddress.String(),
			},
		},
		{
			name: "reAgainRegisterAndApproveContract",
			data: []string{
				"d7e5fbf30000000000000000000000005b38da6a701c568545dcfcb03fcb875f56beddc400000000000000000000000078731d3ca6b7e34ac0f824c42a7cc18a495cabab",
				"07f7aafb0000000000000000000000005b38da6a701c568545dcfcb03fcb875f56beddc4",
			},
			checkContractExpect: checkContractExpectation{
				contractAddress:  registerAddress.String(),
				expectReceiver:   receiverAddress,
				expectPercentage: expectPercentage,
				expectIsApproved: true,
			},
		},
		{
			name: "reDisApproveAndApproveContract",
			data: []string{
				"a0ee93180000000000000000000000005b38da6a701c568545dcfcb03fcb875f56beddc4",
				"a0ee93180000000000000000000000005b38da6a701c568545dcfcb03fcb875f56beddc4",
				"07f7aafb0000000000000000000000005b38da6a701c568545dcfcb03fcb875f56beddc4",
			},
			checkContractExpect: checkContractExpectation{
				contractAddress:  registerAddress.String(),
				expectReceiver:   receiverAddress,
				expectPercentage: expectPercentage,
				expectIsApproved: true,
			},
		},
		{
			name: "removeAndApproveContract",
			data: []string{
				"c375c2ef0000000000000000000000005b38da6a701c568545dcfcb03fcb875f56beddc4",
				"07f7aafb0000000000000000000000005b38da6a701c568545dcfcb03fcb875f56beddc4",
			},
			checkContractExpect: checkContractExpectation{
				contractAddress: registerAddress.String(),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for i := 0; i < len(tt.data); i++ {
				data, err := hex.DecodeString(tt.data[i])
				r.NoError(err)
				exec, err = action.SignedExecution(contractAddress, _execPriKey, atomic.AddUint64(&nonce, 1), big.NewInt(0), 10000000, big.NewInt(9000000000000), data)
				r.NoError(err)
				r.NoError(ap.Add(context.Background(), exec))
				blk, err = bc.MintNewBlock(fixedTime)
				r.NoError(err)
				r.NoError(bc.CommitBlock(blk))
				height, err = dao.Height()
				r.NoError(err)
				ctx = genesis.WithGenesisContext(
					protocol.WithBlockchainCtx(
						protocol.WithRegistry(ctx, registry),
						protocol.BlockchainCtx{
							Tip: protocol.TipInfo{
								Height:    height,
								Hash:      blk.HashHeader(),
								Timestamp: blk.Timestamp(),
							},
						}),
					cfg.Genesis,
				)
				r.NoError(sgdRegistry.PutBlock(ctx, blk))
			}
			receiver, percentage, isApproved, err := sgdRegistry.CheckContract(ctx, tt.checkContractExpect.contractAddress, height)
			if tt.checkContractExpect.errorContains != "" {
				r.ErrorContains(err, tt.checkContractExpect.errorContains)
			} else {
				r.NoError(err)
			}
			r.Equal(tt.checkContractExpect.expectPercentage, percentage)
			r.Equal(tt.checkContractExpect.expectReceiver, receiver)
			r.Equal(tt.checkContractExpect.expectIsApproved, isApproved)

		})
	}

}
