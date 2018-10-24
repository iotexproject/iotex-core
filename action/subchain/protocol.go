// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package subchain

import (
	"context"
	"fmt"
	"math/big"
	"path"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/chainservice"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/dispatcher"
	"github.com/iotexproject/iotex-core/explorer/idl/explorer"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/network"
	"github.com/iotexproject/iotex-core/pkg/enc"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/state"
)

const (
	// MainChainID reserves the ID for main chain
	MainChainID uint32 = 1
)

var (
	// MinSecurityDeposit represents the security deposit minimal required for start a sub-chain, which is 1M iotx
	MinSecurityDeposit = big.NewInt(0).Mul(big.NewInt(1000000000), big.NewInt(blockchain.Iotx))
	// usedChainIDsKey is to find the used chain IDs in the state factory
	usedChainIDsKey = byteutil.BytesTo20B(hash.Hash160b([]byte("usedChainIDs")))
)

// Protocol defines the protocol of handling sub-chain actions
type Protocol struct {
	cfg              *config.Config
	p2p              network.Overlay
	dispatcher       dispatcher.Dispatcher
	rootChain        blockchain.Blockchain
	sf               state.Factory
	rootChainAPI     explorer.Explorer
	subChainServices map[uint32]*chainservice.ChainService
}

// NewProtocol instantiates the protocol of sub-chain
func NewProtocol(
	cfg *config.Config,
	p2p network.Overlay,
	dispatcher dispatcher.Dispatcher,
	rootChain blockchain.Blockchain,
	rootChainAPI explorer.Explorer,
) *Protocol {
	return &Protocol{
		cfg:              cfg,
		p2p:              p2p,
		dispatcher:       dispatcher,
		rootChain:        rootChain,
		sf:               rootChain.GetFactory(),
		rootChainAPI:     rootChainAPI,
		subChainServices: make(map[uint32]*chainservice.ChainService),
	}
}

// Handle handles how to mutate the state db given the sub-chain action
func (p *Protocol) Handle(act action.Action, ws state.WorkingSet) error {
	switch act := act.(type) {
	case *action.StartSubChain:
		if err := p.handleStartSubChain(act, ws); err != nil {
			return errors.Wrapf(err, "error when handling start sub-chain action")
		}
	case *action.PutBlock:
		if err := p.handlePutBlock(act, ws); err != nil {
			return errors.Wrapf(err, "error when handling start sub-chain action")
		}
	}
	// The action is not handled by this handler or no error
	return nil
}

// Validate validates the sub-chain action
func (p *Protocol) Validate(act action.Action) error {
	switch act.(type) {
	case *action.StartSubChain:
		if _, _, err := p.validateStartSubChain(act.(*action.StartSubChain), nil); err != nil {
			return errors.Wrapf(err, "error when handling start sub-chain action")
		}
	}
	// The action is not validated by this handler or no error
	return nil
}

// SubChain returns the confirmed sub-chain state
func (p *Protocol) SubChain(addr address.Address) (*SubChain, error) {
	var subChain SubChain
	state, err := p.sf.State(byteutil.BytesTo20B(addr.Payload()), &subChain)
	if err != nil {
		return nil, errors.Wrapf(err, "error when loading state of %x", addr.Payload())
	}
	sc, ok := state.(*SubChain)
	if !ok {
		return nil, errors.New("error when casting state into sub-chain")
	}
	return sc, nil
}

// UsedChainIDs returns the used chain IDs
func (p *Protocol) UsedChainIDs() (UsedChainIDs, error) {
	var usedChainIDs UsedChainIDs
	s, err := p.sf.State(usedChainIDsKey, &usedChainIDs)
	return processState(s, err)
}

func (p *Protocol) handleStartSubChain(start *action.StartSubChain, ws state.WorkingSet) error {
	account, usedChainIDs, err := p.validateStartSubChain(start, ws)
	if err != nil {
		return err
	}
	subChain, err := p.mutateSubChainState(start, account, usedChainIDs, ws)
	if err != nil {
		return err
	}
	if err := p.startSubChainService(subChain); err != nil {
		return nil
	}
	return nil
}

func (p *Protocol) validateStartSubChain(
	start *action.StartSubChain,
	ws state.WorkingSet,
) (*state.Account, UsedChainIDs, error) {
	if start.ChainID() == MainChainID {
		return nil, nil, fmt.Errorf("%d is used by main chain", start.ChainID())
	}
	var usedChainIDs UsedChainIDs
	var err error
	if ws == nil {
		usedChainIDs, err = p.UsedChainIDs()
	} else {
		usedChainIDs, err = cachedUsedChainIDs(ws)
	}
	if err != nil {
		return nil, nil, errors.Wrap(err, "error when getting the state of used chain IDs")
	}
	if usedChainIDs.Exist(start.ChainID()) {
		return nil, nil, fmt.Errorf("%d is used by another sub-chain", start.ChainID())
	}
	var account *state.Account
	if ws == nil {
		account, err = p.sf.AccountState(start.OwnerAddress())
	} else {
		account, err = ws.CachedAccountState(start.OwnerAddress())
	}
	if err != nil {
		return nil, nil, errors.Wrapf(err, "error when getting the state of address %s", start.OwnerAddress())
	}
	if start.SecurityDeposit().Cmp(MinSecurityDeposit) < 0 {
		return nil, nil, fmt.Errorf(
			"security deposit is smaller than the minimal requirement %s",
			MinSecurityDeposit.String(),
		)
	}
	if account.Balance.Cmp(start.SecurityDeposit()) < 0 {
		return nil, nil, errors.New("sub-chain owner doesn't have enough balance for security deposit")
	}
	if account.Balance.Cmp(big.NewInt(0).Add(start.SecurityDeposit(), start.OperationDeposit())) < 0 {
		return nil, nil, errors.New("sub-chain owner doesn't have enough balance for operation deposit")
	}
	return account, usedChainIDs, nil
}

func (p *Protocol) mutateSubChainState(
	start *action.StartSubChain,
	account *state.Account,
	usedChainIDs UsedChainIDs,
	ws state.WorkingSet,
) (*SubChain, error) {
	addr, err := createSubChainAddress(start.OwnerAddress(), start.Nonce())
	if err != nil {
		return nil, err
	}
	sc := SubChain{
		ChainID:            start.ChainID(),
		SecurityDeposit:    start.SecurityDeposit(),
		OperationDeposit:   start.OperationDeposit(),
		StartHeight:        start.StartHeight(),
		ParentHeightOffset: start.ParentHeightOffset(),
		OwnerPublicKey:     start.OwnerPublicKey(),
		CurrentHeight:      0,
	}
	if err := ws.PutState(addr, &sc); err != nil {
		return nil, errors.Wrap(err, "error when putting sub-chain state")
	}
	account.Balance = big.NewInt(0).Sub(account.Balance, start.SecurityDeposit())
	account.Balance = big.NewInt(0).Sub(account.Balance, start.OperationDeposit())
	// TODO: this is not right, but currently the actions in a block is not processed according to the nonce
	if start.Nonce() > account.Nonce {
		account.Nonce = start.Nonce()
	}
	ownerPKHash, err := ownerAddressPKHash(start.OwnerAddress())
	if err != nil {
		return nil, err
	}
	if err := ws.PutState(ownerPKHash, account); err != nil {
		return nil, err
	}
	usedChainIDs = usedChainIDs.Append(start.ChainID())
	if err := ws.PutState(usedChainIDsKey, &usedChainIDs); err != nil {
		return nil, err
	}
	// TODO: update voting results because of owner account balance change
	return &sc, nil
}

func (p *Protocol) startSubChainService(sc *SubChain) error {
	block := make(chan *blockchain.Block)
	if err := p.rootChain.SubscribeBlockCreation(block); err != nil {
		return errors.Wrap(err, "error when subscribing block creation")
	}

	go func() {
		for started := false; !started; {
			select {
			case blk := <-block:
				if blk.Height() < sc.StartHeight {
					continue
				}
				// TODO: get rid of the hack config modification
				cfg := p.cfg
				cfg.Chain.ID = sc.ChainID
				cfg.Chain.ChainDBPath = getSubChainDBPath(sc.ChainID, cfg.Chain.ChainDBPath)
				cfg.Chain.TrieDBPath = getSubChainDBPath(sc.ChainID, cfg.Chain.TrieDBPath)
				cfg.Explorer.Port = cfg.Explorer.Port - int(MainChainID) + int(sc.ChainID)

				opts := []chainservice.Option{chainservice.WithRootChainAPI(p.rootChainAPI)}
				chainService, err := chainservice.New(p.cfg, p.p2p, p.dispatcher, opts...)
				if err != nil {
					logger.Error().Err(err).Msgf("error when constructing the sub-chain %d", sc.StartHeight)
					continue
				}
				p.subChainServices[sc.ChainID] = chainService
				p.dispatcher.AddSubscriber(sc.ChainID, chainService)
				// TODO: inherit ctx from root chain
				if err := chainService.Start(context.Background()); err != nil {
					logger.Error().Err(err).Msgf("error when starting the sub-chain %d", sc.StartHeight)
					continue
				}
				logger.Info().Msgf("started the sub-chain %d", sc.StartHeight)
				// No matter if the start process failed or not
				started = true
			}
		}
		if err := p.rootChain.UnSubscribeBlockCreation(block); err != nil {
			logger.Error().Err(err).Msg("error when unsubscribing block creation")
		}
	}()

	return nil
}

func (p *Protocol) handlePutBlock(start *action.PutBlock, ws state.WorkingSet) error {
	// TODO
	return nil
}

func createSubChainAddress(ownerAddr string, nonce uint64) (hash.PKHash, error) {
	addr, err := address.IotxAddressToAddress(ownerAddr)
	if err != nil {
		return hash.ZeroPKHash, errors.Wrapf(err, "cannot get the public key hash of address %s", ownerAddr)
	}
	bytes := make([]byte, 8)
	enc.MachineEndian.PutUint64(bytes, nonce)
	return byteutil.BytesTo20B(hash.Hash160b(append(addr.Payload(), bytes...))), nil
}

func ownerAddressPKHash(ownerAddr string) (hash.PKHash, error) {
	addr, err := address.IotxAddressToAddress(ownerAddr)
	if err != nil {
		return hash.ZeroPKHash, errors.Wrapf(err, "cannot get the public key hash of address %s", ownerAddr)
	}
	return byteutil.BytesTo20B(addr.Payload()), nil
}

func getSubChainDBPath(chainID uint32, p string) string {
	dir, file := path.Split(p)
	return path.Join(dir, fmt.Sprintf("chain-%d-%s", chainID, file))
}

func cachedUsedChainIDs(ws state.WorkingSet) (UsedChainIDs, error) {
	var usedChainIDs UsedChainIDs
	s, err := ws.CachedState(usedChainIDsKey, &usedChainIDs)
	return processState(s, err)
}

func processState(s state.State, err error) (UsedChainIDs, error) {
	if err != nil {
		if errors.Cause(err) == state.ErrStateNotExist {
			return UsedChainIDs{}, nil
		}
		return nil, errors.Wrapf(err, "error when loading state of %x", usedChainIDsKey)
	}
	uci, ok := s.(*UsedChainIDs)
	if !ok {
		return nil, errors.New("error when casting state into used chain IDs")
	}
	return *uci, nil
}
