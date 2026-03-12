package ioswarm

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"go.uber.org/zap"
)

// RewardSettler sends on-chain settlement transactions to the AgentRewardPool contract.
type RewardSettler interface {
	// Settle calls depositAndSettle on the reward pool contract.
	// agents: list of 0x-prefixed agent wallet addresses
	// weights: corresponding weight for each agent (same length)
	// value: total IOTX to deposit in wei/rau
	Settle(ctx context.Context, agents []string, weights []*big.Int, value *big.Int) error
}

const depositAndSettleABI = `[{"inputs":[{"internalType":"address[]","name":"_agents","type":"address[]"},{"internalType":"uint256[]","name":"_weights","type":"uint256[]"}],"name":"depositAndSettle","outputs":[],"stateMutability":"payable","type":"function"}]`

// OnChainSettler implements RewardSettler using go-ethereum's ethclient.
type OnChainSettler struct {
	client   *ethclient.Client
	chainID  *big.Int
	contract common.Address
	privKey  *ecdsa.PrivateKey
	abi      abi.ABI
	logger   *zap.Logger

	mu          sync.Mutex
	nonce       uint64
	nonceLoaded bool
}

// NewOnChainSettler creates a settler from Config.
// Returns (nil, nil) if RewardContract or RewardSignerKey is empty.
func NewOnChainSettler(cfg Config, logger *zap.Logger) (*OnChainSettler, error) {
	if cfg.RewardContract == "" || cfg.RewardSignerKey == "" {
		return nil, nil
	}

	rpcURL := cfg.RewardRPCURL
	if rpcURL == "" {
		rpcURL = "https://babel-api.mainnet.iotex.io"
	}
	chainID := int64(4689)
	if cfg.RewardChainID > 0 {
		chainID = cfg.RewardChainID
	}

	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("dial RPC %s: %w", rpcURL, err)
	}

	keyBytes, err := hex.DecodeString(strings.TrimPrefix(cfg.RewardSignerKey, "0x"))
	if err != nil {
		return nil, fmt.Errorf("decode signer key: %w", err)
	}
	privKey, err := crypto.ToECDSA(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("parse signer key: %w", err)
	}

	parsed, err := abi.JSON(strings.NewReader(depositAndSettleABI))
	if err != nil {
		return nil, fmt.Errorf("parse ABI: %w", err)
	}

	return &OnChainSettler{
		client:   client,
		chainID:  big.NewInt(chainID),
		contract: common.HexToAddress(cfg.RewardContract),
		privKey:  privKey,
		abi:      parsed,
		logger:   logger,
	}, nil
}

// Settle calls depositAndSettle(address[], uint256[]) on the reward pool contract.
func (s *OnChainSettler) Settle(ctx context.Context, agents []string, weights []*big.Int, value *big.Int) error {
	if len(agents) != len(weights) {
		return fmt.Errorf("agents/weights length mismatch: %d vs %d", len(agents), len(weights))
	}
	if len(agents) == 0 {
		return nil
	}

	// Convert string addresses to common.Address
	addrs := make([]common.Address, len(agents))
	for i, a := range agents {
		addrs[i] = common.HexToAddress(a)
	}

	// Encode calldata
	data, err := s.abi.Pack("depositAndSettle", addrs, weights)
	if err != nil {
		return fmt.Errorf("pack calldata: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Get nonce (cached, incremented after each send)
	if !s.nonceLoaded {
		from := crypto.PubkeyToAddress(s.privKey.PublicKey)
		nonce, err := s.client.PendingNonceAt(ctx, from)
		if err != nil {
			return fmt.Errorf("get nonce: %w", err)
		}
		s.nonce = nonce
		s.nonceLoaded = true
	}

	// Gas price
	gasPrice, err := s.client.SuggestGasPrice(ctx)
	if err != nil {
		return fmt.Errorf("suggest gas price: %w", err)
	}

	// Gas limit: base 200k + 80k per agent (previous 60k+30k caused OOG reverts)
	gasLimit := uint64(200000 + 80000*len(agents))
	if gasLimit > 8000000 {
		gasLimit = 8000000
	}

	// Build transaction
	tx := types.NewTransaction(
		s.nonce,
		s.contract,
		value,
		gasLimit,
		gasPrice,
		data,
	)

	// Sign with EIP-155
	signer := types.NewEIP155Signer(s.chainID)
	signedTx, err := types.SignTx(tx, signer, s.privKey)
	if err != nil {
		return fmt.Errorf("sign tx: %w", err)
	}

	// Send
	if err := s.client.SendTransaction(ctx, signedTx); err != nil {
		s.nonceLoaded = false
		return fmt.Errorf("send tx: %w", err)
	}

	txHash := signedTx.Hash().Hex()
	s.nonce++

	s.logger.Info("depositAndSettle tx sent",
		zap.String("tx_hash", txHash),
		zap.Int("agents", len(agents)),
		zap.String("value_iotx", FormatIOTX(value)))

	// Wait for receipt in background
	go s.waitForReceipt(txHash, signedTx.Hash())

	return nil
}

func (s *OnChainSettler) waitForReceipt(txHashStr string, txHash common.Hash) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			s.logger.Warn("timeout waiting for receipt", zap.String("tx_hash", txHashStr))
			return
		case <-time.After(3 * time.Second):
			receipt, err := s.client.TransactionReceipt(ctx, txHash)
			if err != nil {
				continue // not mined yet
			}
			if receipt.Status == types.ReceiptStatusSuccessful {
				s.logger.Info("depositAndSettle confirmed",
					zap.String("tx_hash", txHashStr),
					zap.Uint64("gas_used", receipt.GasUsed))
			} else {
				s.logger.Error("depositAndSettle reverted",
					zap.String("tx_hash", txHashStr),
					zap.Uint64("gas_used", receipt.GasUsed))
			}
			return
		}
	}
}

// IOTXToWei converts IOTX float to wei (1 IOTX = 10^18 wei).
func IOTXToWei(iotx float64) *big.Int {
	f := new(big.Float).SetFloat64(iotx)
	exp := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
	f.Mul(f, exp)
	wei, _ := f.Int(nil)
	return wei
}
