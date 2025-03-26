package poll

import (
	"context"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/execution/evm"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/state"
)

var (
	_consortiumCommitteeContractCreator = address.ZeroAddress
	_consortiumCommitteeContractNonce   = uint64(0)
	// this is a special execution that is not signed, set hash = hex-string of "_consortiumCommitteeContractHash"
	_consortiumCommitteeContractHash, _ = hash.HexStringToHash256("00636f6e736f727469756d436f6d6d6974746565436f6e747261637448617368")
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
}

// NewConsortiumCommittee creates a committee for consorium chain
// TODO: remove unused getBlockHash
func NewConsortiumCommittee(indexer *CandidateIndexer, readContract ReadContract, _ evm.GetBlockHash) (Protocol, error) {
	abi, err := abi.JSON(strings.NewReader(ConsortiumManagementABI))
	if err != nil {
		return nil, err
	}

	if readContract == nil {
		return nil, errors.New("failed to create consortium committee: empty read contract callback")
	}
	h := hash.Hash160b([]byte(_protocolID))
	addr, err := address.FromBytes(h[:])
	if err != nil {
		return nil, err
	}
	return &consortiumCommittee{
		contractReader: genContractReaderFromReadContract(readContract, true),
		abi:            abi,
		addr:           addr,
		indexer:        indexer,
	}, nil
}

func (cc *consortiumCommittee) Start(ctx context.Context, sr protocol.StateReader) (interface{}, error) {
	g := genesis.MustExtractGenesisContext(ctx)
	if g.ConsortiumCommitteeContractCode == "" {
		return nil, errors.New("cannot find consortium committee contract in gensis")
	}

	caller, _ := address.FromString(_consortiumCommitteeContractCreator)
	ethAddr := crypto.CreateAddress(common.BytesToAddress(caller.Bytes()), _consortiumCommitteeContractNonce)
	iotxAddr, _ := address.FromBytes(ethAddr.Bytes())
	cc.contract = iotxAddr.String()
	log.L().Debug("Loaded consortium committee contract", zap.String("address", iotxAddr.String()))

	return nil, nil
}

func (cc *consortiumCommittee) CreateGenesisStates(ctx context.Context, sm protocol.StateManager) error {
	g := genesis.MustExtractGenesisContext(ctx)
	blkCtx := protocol.MustGetBlockCtx(ctx)
	blkCtx.Producer, _ = address.FromString(_consortiumCommitteeContractCreator)
	blkCtx.GasLimit = g.BlockGasLimitByHeight(0)
	bytes, err := hexutil.Decode(g.ConsortiumCommitteeContractCode)
	if err != nil {
		return err
	}
	execution := action.NewExecution("", big.NewInt(0), bytes)
	elp := (&action.EnvelopeBuilder{}).SetGasLimit(g.BlockGasLimitByHeight(0)).
		SetNonce(_consortiumCommitteeContractNonce).SetAction(execution).Build()
	actionCtx := protocol.ActionCtx{}
	actionCtx.Caller, err = address.FromString(_consortiumCommitteeContractCreator)
	if err != nil {
		return err
	}
	actionCtx.Nonce = _consortiumCommitteeContractNonce
	actionCtx.ActionHash = _consortiumCommitteeContractHash
	actionCtx.GasPrice = elp.GasPrice()
	actionCtx.IntrinsicGas, err = elp.IntrinsicGas()
	if err != nil {
		return err
	}
	ctx = protocol.WithActionCtx(ctx, actionCtx)
	ctx = protocol.WithBlockCtx(ctx, blkCtx)
	getBlockTime := func(u uint64) (time.Time, error) {
		// make sure the returned timestamp is after the current block time so that evm upgrades based on timestamp (Shanghai and onwards) are disabled
		return blkCtx.BlockTimeStamp.Add(5 * time.Second), nil
	}
	ctx = evm.WithHelperCtx(ctx, evm.HelperContext{
		GetBlockHash: func(height uint64) (hash.Hash256, error) {
			return hash.ZeroHash256, nil
		},
		GetBlockTime: getBlockTime,
		DepositGasFunc: func(context.Context, protocol.StateManager, *big.Int, ...protocol.DepositOption) ([]*action.TransactionLog, error) {
			return nil, nil
		},
	})

	// deploy consortiumCommittee contract
	_, receipt, err := evm.ExecuteContract(ctx, sm, elp)
	if err != nil {
		return err
	}
	if receipt.Status != uint64(iotextypes.ReceiptStatus_Success) {
		return errors.Errorf("error when deploying consortiumCommittee contract, status=%d", receipt.Status)
	}
	cc.contract = receipt.ContractAddress

	ctx = evm.WithHelperCtx(ctx, evm.HelperContext{
		GetBlockHash: protocol.MustGetBlockchainCtx(ctx).GetBlockHash,
		GetBlockTime: getBlockTime,
	})
	r := getContractReaderForGenesisStates(ctx, sm)
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

func (cc *consortiumCommittee) Handle(ctx context.Context, elp action.Envelope, sm protocol.StateManager) (*action.Receipt, error) {
	return handle(ctx, elp.Action(), sm, cc.indexer, cc.addr.String())
}

func (cc *consortiumCommittee) Validate(ctx context.Context, elp action.Envelope, sr protocol.StateReader) error {
	return validate(ctx, sr, cc, elp.Action())
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
	return r.Register(_protocolID, cc)
}

// ForceRegister registers the protocol with a unique ID and force replacing the previous protocol if it exists
func (cc *consortiumCommittee) ForceRegister(r *protocol.Registry) error {
	return r.ForceRegister(_protocolID, cc)
}

// Name returns the name of protocol
func (cc *consortiumCommittee) Name() string {
	return _protocolID
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
	ret, err := cc.abi.Unpack("delegates", data)
	if err != nil {
		return nil, err
	}
	res, err := toEtherAddressSlice(ret[0])
	if err != nil {
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

func getContractReaderForGenesisStates(ctx context.Context, sm protocol.StateManager) contractReaderFunc {
	return func(ctx context.Context, contract string, data []byte) ([]byte, error) {
		gasLimit := uint64(10000000)
		ex := action.NewExecution(contract, big.NewInt(0), data)

		addr, err := address.FromString(address.ZeroAddress)
		if err != nil {
			return nil, err
		}

		elp := (&action.EnvelopeBuilder{}).SetNonce(1).SetGasLimit(gasLimit).
			SetAction(ex).Build()
		res, _, err := evm.SimulateExecution(ctx, sm, addr, elp)
		return res, err
	}
}

func toEtherAddressSlice(v interface{}) ([]common.Address, error) {
	if addr, ok := v.([]common.Address); ok {
		return addr, nil
	}
	return nil, ErrWrongData
}
