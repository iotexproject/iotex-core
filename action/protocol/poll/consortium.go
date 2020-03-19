package poll

import (
	"context"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/execution/evm"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type consortiumCommittee struct {
	readContract ReadContract
	contract     string
	abi          abi.ABI
	bufferHeight uint64
	bufferResult state.CandidateList
	indexer      *CandidateIndexer
	addr         address.Address
}

func NewConsortiumCommittee(indexer *CandidateIndexer, readContract ReadContract) (*consortiumCommittee, error) {
	abi, err := abi.JSON(strings.NewReader(ConsortiumManagementABI))
	if err != nil {
		return nil, err
	}

	if readContract == nil {
		return nil, errors.New("failed to create native staking: empty read contract callback")
	}
	h := hash.Hash160b([]byte(protocolID))
	addr, err := address.FromBytes(h[:])
	if err != nil {
		return nil, err
	}
	return &consortiumCommittee{
		readContract: readContract,
		abi:          abi,
		addr:         addr,
		indexer:      indexer,
	}, nil
}

func (cc *consortiumCommittee) Start(ctx context.Context) error {
	//bcCtx := protocol.MustGetBlockchainCtx(ctx)
	//if bcCtx.Genesis.ConsortiumCommitteeContractCode == "" {
	//return errors.New("cannot find consortium committee contract in gensis")
	//}

	caller, _ := address.FromString(nativeStakingContractCreator)
	ethAddr := crypto.CreateAddress(common.BytesToAddress(caller.Bytes()), nativeStakingContractNonce)
	iotxAddr, _ := address.FromBytes(ethAddr.Bytes())
	cc.contract = iotxAddr.String()
	log.L().Info("Loaded consortium committee contract", zap.String("address", iotxAddr.String()))

	return nil
}

func (cc *consortiumCommittee) CreateGenesisStates(ctx context.Context, sm protocol.StateManager) error {
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	blkCtx := protocol.MustGetBlockCtx(ctx)
	blkCtx.Producer, _ = address.FromString(address.ZeroAddress)
	blkCtx.GasLimit = bcCtx.Genesis.BlockGasLimit
	bytes, err := hexutil.Decode(bcCtx.Genesis.NativeStakingContractCode)
	if err != nil {
		return err
	}
	execution, err := action.NewExecution(
		"",
		nativeStakingContractNonce,
		big.NewInt(0),
		bcCtx.Genesis.BlockGasLimit,
		big.NewInt(0),
		bytes,
	)
	if err != nil {
		return err
	}
	actionCtx := protocol.ActionCtx{}
	actionCtx.Caller, err = address.FromString(nativeStakingContractCreator)
	if err != nil {
		return err
	}
	actionCtx.Nonce = nativeStakingContractNonce
	actionCtx.ActionHash = execution.Hash()
	actionCtx.GasPrice = execution.GasPrice()
	actionCtx.IntrinsicGas, err = execution.IntrinsicGas()
	if err != nil {
		return err
	}
	ctx = protocol.WithActionCtx(ctx, actionCtx)
	ctx = protocol.WithBlockCtx(ctx, blkCtx)
	// deploy native staking contract
	_, receipt, err := evm.ExecuteContract(
		ctx,
		sm,
		execution,
		func(height uint64) (hash.Hash256, error) {
			return hash.ZeroHash256, nil
		},
	)
	if err != nil {
		return err
	}
	if receipt.Status != uint64(iotextypes.ReceiptStatus_Success) {
		return errors.Errorf("error when deploying native staking contract, status=%d", receipt.Status)
	}
	cc.contract = receipt.ContractAddress

	cands, err := cc.readCandidates(ctx)
	if err != nil {
		return err
	}

	return setCandidates(ctx, sm, cc.indexer, cands, uint64(1))
}

func (cc *consortiumCommittee) CreatePreStates(ctx context.Context, sm protocol.StateManager) error {
	return nil
}

func (cc *consortiumCommittee) CreatePostSystemActions(ctx context.Context) ([]action.Envelope, error) {
	return createPostSystemActions(ctx, cc)
}

func (cc *consortiumCommittee) Handle(ctx context.Context, act action.Action, sm protocol.StateManager) (*action.Receipt, error) {
	return handle(ctx, act, sm, cc.indexer, cc.addr.String())
}

func (cc *consortiumCommittee) Validate(ctx context.Context, act action.Action) error {
	return validate(ctx, cc, act)
}

func (cc *consortiumCommittee) ReadState(
	ctx context.Context,
	sm protocol.StateReader,
	method []byte,
	args ...[]byte,
) ([]byte, error) {
	return nil, nil
}

// Register registers the protocol with a unique ID
func (cc *consortiumCommittee) Register(r *protocol.Registry) error {
	return r.Register(protocolID, cc)
}

// ForceRegister registers the protocol with a unique ID and force replacing the previous protocol if it exists
func (cc *consortiumCommittee) ForceRegister(r *protocol.Registry) error {
	return r.ForceRegister(protocolID, cc)
}

func (cc *consortiumCommittee) CalculateCandidatesByHeight(ctx context.Context, height uint64) (state.CandidateList, error) {
	return cc.readCandidates(ctx)
}

func (cc *consortiumCommittee) DelegatesByEpoch(ctx context.Context, epochNum uint64) (state.CandidateList, error) {
	return cc.readCandidates(ctx)
}

func (cc *consortiumCommittee) CandidatesByHeight(ctx context.Context, height uint64) (state.CandidateList, error) {
	return cc.readCandidates(ctx)
}

func (cc *consortiumCommittee) readCandidates(ctx context.Context) (state.CandidateList, error) {
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	if cc.bufferHeight == bcCtx.Tip.Height && cc.bufferResult != nil {
		return cc.bufferResult, nil
	}

	data, err := cc.abi.Pack("delegates")
	if err != nil {
		return nil, err
	}

	data, err = cc.readContract(ctx, cc.contract, data, true)
	if err != nil {
		return nil, err
	}
	var res []common.Address
	if err = cc.abi.Unpack(res, "delegates", data); err != nil {
		return nil, err
	}

	candidates := make([]*state.Candidate, 0, len(res))
	for _, ethAddr := range res {
		addr, err := toIoAddress(ethAddr)
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, &state.Candidate{
			Address: addr.String(),
		})
	}
	cc.bufferHeight = bcCtx.Tip.Height
	cc.bufferResult = candidates
	return candidates, nil
}

func toIoAddress(addr common.Address) (address.Address, error) {
	pkhash, err := hexutil.Decode(addr.String())
	if err != nil {
		return nil, err
	}
	return address.FromBytes(pkhash)
}
