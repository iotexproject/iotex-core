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
	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

var (
	consortiumCommitteeContractCreator = address.ZeroAddress
	consortiumCommitteeContractNonce   = uint64(0)
	// this is a special execution that is not signed, set hash = hex-string of "consortiumCommitteeContractHash"
	consortiumCommitteeContractHash, _ = hash.HexStringToHash256("00636f6e736f727469756d436f6d6d6974746565436f6e747261637448617368")
)

type contractReader interface {
	Read(ctx context.Context, contract string, data []byte) ([]byte, error)
}

type contractReaderFunc func(context.Context, string, []byte) ([]byte, error)

func (f contractReaderFunc) Read(ctx context.Context, contract string, data []byte) ([]byte, error) {
	return f(ctx, contract, data)
}

type consortiumCommittee struct {
	contractReader contractReader
	contract       string
	abi            abi.ABI
	bufferEpochNum uint64
	bufferResult   state.CandidateList
	indexer        *CandidateIndexer
	addr           address.Address
	getBlockHash   evm.GetBlockHash
}

// NewConsortiumCommittee creates a committee for consorium chain
func NewConsortiumCommittee(indexer *CandidateIndexer, readContract ReadContract, getBlockHash evm.GetBlockHash) (Protocol, error) {
	abi, err := abi.JSON(strings.NewReader(ConsortiumManagementABI))
	if err != nil {
		return nil, err
	}

	if readContract == nil {
		return nil, errors.New("failed to create consortium committee: empty read contract callback")
	}
	h := hash.Hash160b([]byte(protocolID))
	addr, err := address.FromBytes(h[:])
	if err != nil {
		return nil, err
	}
	return &consortiumCommittee{
		contractReader: genContractReaderFromReadContract(readContract, true),
		abi:            abi,
		addr:           addr,
		indexer:        indexer,
		getBlockHash:   getBlockHash,
	}, nil
}

func (cc *consortiumCommittee) Start(ctx context.Context, sr protocol.StateReader) (interface{}, error) {
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	if bcCtx.Genesis.ConsortiumCommitteeContractCode == "" {
		return nil, errors.New("cannot find consortium committee contract in gensis")
	}

	caller, _ := address.FromString(consortiumCommitteeContractCreator)
	ethAddr := crypto.CreateAddress(common.BytesToAddress(caller.Bytes()), consortiumCommitteeContractNonce)
	iotxAddr, _ := address.FromBytes(ethAddr.Bytes())
	cc.contract = iotxAddr.String()
	log.L().Info("Loaded consortium committee contract", zap.String("address", iotxAddr.String()))

	return nil, nil
}

func (cc *consortiumCommittee) CreateGenesisStates(ctx context.Context, sm protocol.StateManager) error {
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	blkCtx := protocol.MustGetBlockCtx(ctx)
	blkCtx.Producer, _ = address.FromString(consortiumCommitteeContractCreator)
	blkCtx.GasLimit = bcCtx.Genesis.BlockGasLimit
	bytes, err := hexutil.Decode(bcCtx.Genesis.ConsortiumCommitteeContractCode)
	if err != nil {
		return err
	}
	execution, err := action.NewExecution(
		"",
		consortiumCommitteeContractNonce,
		big.NewInt(0),
		bcCtx.Genesis.BlockGasLimit,
		big.NewInt(0),
		bytes,
	)
	if err != nil {
		return err
	}
	actionCtx := protocol.ActionCtx{}
	actionCtx.Caller, err = address.FromString(consortiumCommitteeContractCreator)
	if err != nil {
		return err
	}
	actionCtx.Nonce = consortiumCommitteeContractNonce
	actionCtx.ActionHash = consortiumCommitteeContractHash
	actionCtx.GasPrice = execution.GasPrice()
	actionCtx.IntrinsicGas, err = execution.IntrinsicGas()
	if err != nil {
		return err
	}
	ctx = protocol.WithActionCtx(ctx, actionCtx)
	ctx = protocol.WithBlockCtx(ctx, blkCtx)
	// deploy consortiumCommittee contract
	_, receipt, err := evm.ExecuteContract(
		ctx,
		sm,
		execution,
		func(height uint64) (hash.Hash256, error) {
			return hash.ZeroHash256, nil
		},
		func(ctx context.Context, sm protocol.StateManager, amount *big.Int) (*action.TransactionLog, error) {
			return nil, nil
		},
	)
	if err != nil {
		return err
	}
	if receipt.Status != uint64(iotextypes.ReceiptStatus_Success) {
		return errors.Errorf("error when deploying consortiumCommittee contract, status=%d", receipt.Status)
	}
	cc.contract = receipt.ContractAddress

	r := getContractReaderForGenesisStates(ctx, sm, cc.getBlockHash)
	cands, err := cc.readDelegatesWithContractReader(ctx, r)
	if err != nil {
		return err
	}

	return setCandidates(ctx, sm, cc.indexer, cands, uint64(1))
}

func (cc *consortiumCommittee) CreatePreStates(ctx context.Context, sm protocol.StateManager) error {
	return nil
}

func (cc *consortiumCommittee) CreatePostSystemActions(ctx context.Context, sr protocol.StateReader) ([]action.Envelope, error) {
	return createPostSystemActions(ctx, sr, cc)
}

func (cc *consortiumCommittee) Handle(ctx context.Context, act action.Action, sm protocol.StateManager) (*action.Receipt, error) {
	return handle(ctx, act, sm, cc.indexer, cc.addr.String())
}

func (cc *consortiumCommittee) Validate(ctx context.Context, act action.Action, sr protocol.StateReader) error {
	return validate(ctx, sr, cc, act)
}

func (cc *consortiumCommittee) ReadState(
	ctx context.Context,
	sm protocol.StateReader,
	method []byte,
	args ...[]byte,
) ([]byte, uint64, error) {
	return nil, uint64(0), nil
}

// Register registers the protocol with a unique ID
func (cc *consortiumCommittee) Register(r *protocol.Registry) error {
	return r.Register(protocolID, cc)
}

// ForceRegister registers the protocol with a unique ID and force replacing the previous protocol if it exists
func (cc *consortiumCommittee) ForceRegister(r *protocol.Registry) error {
	return r.ForceRegister(protocolID, cc)
}

// Name returns the name of protocol
func (cc *consortiumCommittee) Name() string {
	return protocolID
}

func (cc *consortiumCommittee) CalculateCandidatesByHeight(ctx context.Context, _ protocol.StateReader, _ uint64) (state.CandidateList, error) {
	return cc.readDelegates(ctx)
}

func (cc *consortiumCommittee) CalculateUnproductiveDelegates(
	ctx context.Context,
	sr protocol.StateReader,
) ([]string, error) {
	return nil, nil
}

func (cc *consortiumCommittee) Delegates(ctx context.Context, _ protocol.StateReader) (state.CandidateList, error) {
	return cc.readDelegates(ctx)
}

func (cc *consortiumCommittee) NextDelegates(ctx context.Context, _ protocol.StateReader) (state.CandidateList, error) {
	return cc.readDelegates(ctx)
}

func (cc *consortiumCommittee) Candidates(ctx context.Context, _ protocol.StateReader) (state.CandidateList, error) {
	return cc.readDelegates(ctx)
}

func (cc *consortiumCommittee) NextCandidates(ctx context.Context, _ protocol.StateReader) (state.CandidateList, error) {
	return cc.readDelegates(ctx)
}

func (cc *consortiumCommittee) readDelegates(ctx context.Context) (state.CandidateList, error) {
	return cc.readDelegatesWithContractReader(ctx, cc.contractReader)
}

func (cc *consortiumCommittee) readDelegatesWithContractReader(ctx context.Context, r contractReader) (state.CandidateList, error) {
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	rp := rolldpos.MustGetProtocol(protocol.MustGetRegistry(ctx))
	epochNum := rp.GetEpochNum(bcCtx.Tip.Height)
	if cc.bufferEpochNum == epochNum && cc.bufferResult != nil {
		return cc.bufferResult, nil
	}

	data, err := cc.abi.Pack("delegates")
	if err != nil {
		return nil, err
	}

	data, err = r.Read(ctx, cc.contract, data)
	if err != nil {
		return nil, err
	}
	var res []common.Address
	if err = cc.abi.Unpack(&res, "delegates", data); err != nil {
		return nil, err
	}

	candidates := make([]*state.Candidate, 0, len(res))
	for _, ethAddr := range res {
		pkhash, err := hexutil.Decode(ethAddr.String())
		if err != nil {
			return nil, err
		}
		addr, err := address.FromBytes(pkhash)
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, &state.Candidate{
			Address: addr.String(),
			Votes:   big.NewInt(100),
		})
	}
	cc.bufferEpochNum = epochNum
	cc.bufferResult = candidates
	return candidates, nil
}

func genContractReaderFromReadContract(r ReadContract, setting bool) contractReaderFunc {
	return func(ctx context.Context, contract string, data []byte) ([]byte, error) {
		return r(ctx, contract, data, setting)
	}
}

func getContractReaderForGenesisStates(ctx context.Context, sm protocol.StateManager, getBlockHash evm.GetBlockHash) contractReaderFunc {
	return func(ctx context.Context, contract string, data []byte) ([]byte, error) {
		gasLimit := uint64(10000000)
		ex, err := action.NewExecution(contract, 1, big.NewInt(0), gasLimit, big.NewInt(0), data)
		if err != nil {
			return nil, err
		}

		addr, err := address.FromString(address.ZeroAddress)
		if err != nil {
			return nil, err
		}

		res, _, err := evm.SimulateExecution(ctx, sm, addr, ex, getBlockHash)

		return res, err
	}
}
