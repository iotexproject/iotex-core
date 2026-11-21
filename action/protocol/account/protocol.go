// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package account

import (
	"context"
	"math/big"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/state"
)

// protocolID is the protocol ID
// TODO: it works only for one instance per protocol definition now
const protocolID = "account"

// Protocol defines the protocol of handling account
type Protocol struct {
	addr       address.Address
	depositGas protocol.DepositGas
}

// NewProtocol instantiates the protocol of account
func NewProtocol(depositGas protocol.DepositGas) *Protocol {
	h := hash.Hash160b([]byte(protocolID))
	addr, err := address.FromBytes(h[:])
	if err != nil {
		log.L().Panic("Error when constructing the address of account protocol", zap.Error(err))
	}

	return &Protocol{addr: addr, depositGas: depositGas}
}

// ProtocolAddr returns the address generated from protocol id
func ProtocolAddr() address.Address {
	return protocol.HashStringToAddress(protocolID)
}

// FindProtocol finds the registered protocol from registry
func FindProtocol(registry *protocol.Registry) *Protocol {
	if registry == nil {
		return nil
	}
	p, ok := registry.Find(protocolID)
	if !ok {
		return nil
	}
	ap, ok := p.(*Protocol)
	if !ok {
		log.S().Panic("fail to cast account protocol")
	}
	return ap
}

// Handle handles an account
func (p *Protocol) Handle(ctx context.Context, elp action.Envelope, sm protocol.StateManager) (*action.Receipt, error) {
	if _, ok := elp.Action().(*action.Transfer); ok {
		return p.handleTransfer(ctx, elp, sm)
	}
	return nil, nil
}

// Validate validates an account action
func (p *Protocol) Validate(ctx context.Context, elp action.Envelope, sr protocol.StateReader) error {
	if _, ok := elp.Action().(*action.Transfer); ok {
		if err := p.validateTransfer(ctx, elp); err != nil {
			return errors.Wrap(err, "error when validating transfer action")
		}
	}
	return nil
}

// ReadState read the state on blockchain via protocol
func (p *Protocol) ReadState(context.Context, protocol.StateReader, []byte, ...[]byte) ([]byte, uint64, error) {
	return nil, uint64(0), protocol.ErrUnimplemented
}

// Register registers the protocol with a unique ID
func (p *Protocol) Register(r *protocol.Registry) error {
	return r.Register(protocolID, p)
}

// ForceRegister registers the protocol with a unique ID and force replacing the previous protocol if it exists
func (p *Protocol) ForceRegister(r *protocol.Registry) error {
	return r.ForceRegister(protocolID, p)
}

// Name returns the name of protocol
func (p *Protocol) Name() string {
	return protocolID
}

func createAccount(sm protocol.StateManager, encodedAddr string, init *big.Int, opts ...state.AccountCreationOption) error {
	account := &state.Account{}
	addr, err := address.FromString(encodedAddr)
	if err != nil {
		return errors.Wrap(err, "failed to get address public key hash from encoded address")
	}
	addrHash := hash.BytesToHash160(addr.Bytes())
	_, err = sm.State(account, protocol.LegacyKeyOption(addrHash))
	switch errors.Cause(err) {
	case nil:
		return errors.Errorf("failed to create account %s", encodedAddr)
	case state.ErrStateNotExist:
		account, err := state.NewAccount(opts...)
		if err != nil {
			return err
		}
		if err := account.AddBalance(init); err != nil {
			return errors.Wrapf(err, "failed to add balance %s", init)
		}
		if _, err := sm.PutState(account, protocol.LegacyKeyOption(addrHash)); err != nil {
			return errors.Wrapf(err, "failed to put state for account %x", addrHash)
		}
		return nil
	}
	return err
}

// CreateGenesisStates initializes the protocol by setting the initial balances to some addresses
func (p *Protocol) CreateGenesisStates(ctx context.Context, sm protocol.StateManager) error {
	blkCtx := protocol.MustGetBlockCtx(ctx)
	g := genesis.MustExtractGenesisContext(ctx)
	if err := p.assertZeroBlockHeight(blkCtx.BlockHeight); err != nil {
		return err
	}
	addrs, amounts := g.InitBalances()
	if err := p.assertEqualLength(addrs, amounts); err != nil {
		return err
	}
	if err := p.assertAmounts(amounts); err != nil {
		return err
	}
	opts := []state.AccountCreationOption{}
	if protocol.MustGetFeatureCtx(ctx).CreateLegacyNonceAccount {
		opts = append(opts, state.LegacyNonceAccountTypeOption())
	}
	for i, addr := range addrs {
		if err := createAccount(sm, addr.String(), amounts[i], opts...); err != nil {
			return err
		}
	}
	return nil
}

// CreatePreStates credits the testnet balance grant scheduled at this height, if
// there is one, before any action in the block executes. See
// genesis.TestnetGrants; it is refused on mainnet.
//
// Writes go through the state manager rather than straight at the trie, so they
// reach the Erigon secondary store too. There is no transaction log, since
// nothing spends -- indexers deriving balances from transfer logs alone will not
// see the credit.
//
// A grant can only be scheduled at a height that has not happened yet, which on
// any live chain is past Okhotsk, so a recipient that does not exist yet is
// created with the default zero-nonce account type. No
// LegacyNonceAccountTypeOption here, unlike the paths that also replay
// pre-Okhotsk history.
func (p *Protocol) CreatePreStates(ctx context.Context, sm protocol.StateManager) error {
	blkCtx := protocol.MustGetBlockCtx(ctx)
	g := genesis.MustExtractGenesisContext(ctx)
	addrs, amounts, err := g.GrantsAtHeight(blkCtx.BlockHeight)
	if err != nil {
		return err
	}
	for i, addr := range addrs {
		acct, err := accountutil.LoadOrCreateAccount(sm, addr)
		if err != nil {
			return errors.Wrapf(err, "failed to load account %s for testnet grant", addr)
		}
		if err := acct.AddBalance(amounts[i]); err != nil {
			return errors.Wrapf(err, "failed to credit %s to %s", amounts[i], addr)
		}
		// ValidateTestnetGrants bounds the configured amount, but the balance it
		// lands on is only known here. Over the bound the Erigon store panics in
		// uint256.MustFromBig; returning an error rejects the block instead.
		if acct.Balance.BitLen() > genesis.MaxBalanceBits {
			return errors.Errorf(
				"testnet grant of %s puts %s over %d bits of balance",
				amounts[i], addr, genesis.MaxBalanceBits)
		}
		if err := accountutil.StoreAccount(sm, addr, acct); err != nil {
			return errors.Wrapf(err, "failed to store account %s after testnet grant", addr)
		}
		log.L().Warn("credited testnet balance grant",
			zap.Uint64("height", blkCtx.BlockHeight),
			zap.String("address", addr.String()),
			zap.String("amount", amounts[i].String()),
			zap.String("balance", acct.Balance.String()),
		)
	}
	return nil
}

func (p *Protocol) assertZeroBlockHeight(height uint64) error {
	if height != 0 {
		return errors.Errorf("current block height %d is not zero", height)
	}
	return nil
}

func (p *Protocol) assertEqualLength(addrs []address.Address, amounts []*big.Int) error {
	if len(addrs) != len(amounts) {
		return errors.Errorf(
			"address slice length %d and amounts slice length %d don't match",
			len(addrs),
			len(amounts),
		)
	}
	return nil
}

func (p *Protocol) assertAmounts(amounts []*big.Int) error {
	for _, amount := range amounts {
		if amount.Cmp(big.NewInt(0)) < 0 {
			return errors.Errorf("account amount %s shouldn't be negative", amount.String())
		}
	}
	return nil
}
