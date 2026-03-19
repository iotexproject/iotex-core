package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action/protocol/execution/evm"
)

type (
	rpcClient struct {
		endpoint string
		client   *http.Client
	}

	rpcRequest struct {
		JSONRPC string `json:"jsonrpc"`
		ID      int    `json:"id"`
		Method  string `json:"method"`
		Params  any    `json:"params"`
	}

	rpcError struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}

	rpcResponse struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      int             `json:"id"`
		Result  json.RawMessage `json:"result"`
		Error   *rpcError       `json:"error"`
	}

	blockResult struct {
		Number       string   `json:"number"`
		Hash         string   `json:"hash"`
		Transactions []string `json:"transactions"`
	}

	blockWitnessResult struct {
		Summary      *witnessSummary     `json:"summary"`
		Transactions []txWitnessEnvelope `json:"transactions"`
	}

	witnessSummary struct {
		Contracts  uint64 `json:"contracts"`
		Entries    uint64 `json:"entries"`
		ProofNodes uint64 `json:"proofNodes"`
		ProofBytes uint64 `json:"proofBytes"`
	}

	txWitnessEnvelope struct {
		TxHash     string            `json:"txHash"`
		Contracts  uint64            `json:"contracts"`
		Entries    uint64            `json:"entries"`
		ProofNodes uint64            `json:"proofNodes"`
		ProofBytes uint64            `json:"proofBytes"`
		Witnesses  []contractWitness `json:"witnesses"`
	}

	contractWitness struct {
		Address     string              `json:"address"`
		StorageRoot string              `json:"storageRoot"`
		Entries     []contractWitnessKV `json:"entries"`
		ProofNodes  []string            `json:"proofNodes"`
	}

	contractWitnessKV struct {
		Key   string `json:"key"`
		Value string `json:"value,omitempty"`
	}
)

var (
	_rpcEndpoint  string
	_pollInterval time.Duration
	_startHeight  uint64
	_once         bool
)

func parseFlags(args []string) error {
	fs := flag.NewFlagSet("statelessdemo", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.StringVar(&_rpcEndpoint, "rpc", "http://127.0.0.1:15024", "producer Web3 RPC endpoint")
	fs.DurationVar(&_pollInterval, "poll-interval", 2*time.Second, "poll interval for new blocks")
	fs.Uint64Var(&_startHeight, "start-height", 0, "first height to verify; 0 starts from current tip + 1 unless -once is set")
	fs.BoolVar(&_once, "once", false, "verify a single height and exit")
	fs.Usage = func() {
		_, _ = fmt.Fprintf(os.Stderr, "usage: statelessdemo [-rpc http://127.0.0.1:15024] [-start-height N] [-once]\n")
		fs.PrintDefaults()
	}
	return fs.Parse(args)
}

func main() {
	if err := parseFlags(os.Args[1:]); err != nil {
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	client := &rpcClient{
		endpoint: strings.TrimRight(_rpcEndpoint, "/"),
		client: &http.Client{
			Timeout: 20 * time.Second,
		},
	}

	nextHeight, err := initialHeight(ctx, client, _startHeight, _once)
	if err != nil {
		fatalf("failed to determine starting height: %v", err)
	}
	if err := runVerifier(ctx, client, nextHeight); err != nil && !errors.Is(err, context.Canceled) {
		fatalf("stateless verifier failed: %v", err)
	}
}

func runVerifier(ctx context.Context, client *rpcClient, nextHeight uint64) error {
	ticker := time.NewTicker(_pollInterval)
	defer ticker.Stop()

	for {
		latest, err := client.blockNumber(ctx)
		if err != nil {
			return err
		}
		for nextHeight <= latest {
			if err := validateBlock(ctx, client, nextHeight); err != nil {
				return err
			}
			if _once {
				return nil
			}
			nextHeight++
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func validateBlock(ctx context.Context, client *rpcClient, height uint64) error {
	block, err := client.blockByNumber(ctx, height)
	if err != nil {
		return errors.Wrapf(err, "failed to fetch block %d", height)
	}
	witnessResult, err := client.blockWitnessByNumber(ctx, height)
	if err != nil {
		return errors.Wrapf(err, "failed to fetch block witness %d", height)
	}
	if err := validateBlockWitness(block, witnessResult); err != nil {
		return errors.Wrapf(err, "failed to validate witness for block %d", height)
	}

	summary := witnessResult.Summary
	if summary == nil {
		summary = &witnessSummary{}
	}
	fmt.Printf("validated block %d hash=%s txs=%d contracts=%d entries=%d proofNodes=%d proofBytes=%d\n",
		height,
		block.Hash,
		len(block.Transactions),
		summary.Contracts,
		summary.Entries,
		summary.ProofNodes,
		summary.ProofBytes,
	)
	return nil
}

func validateBlockWitness(block *blockResult, result *blockWitnessResult) error {
	if block == nil {
		return errors.New("block is nil")
	}
	if result == nil {
		return errors.New("block witness result is nil")
	}
	if len(result.Transactions) != len(block.Transactions) {
		return errors.Errorf("block transaction count mismatch: block=%d witness=%d", len(block.Transactions), len(result.Transactions))
	}

	var aggregate witnessSummary
	for i, tx := range result.Transactions {
		if !strings.EqualFold(tx.TxHash, block.Transactions[i]) {
			return errors.Errorf("transaction hash mismatch at index %d: block=%s witness=%s", i, block.Transactions[i], tx.TxHash)
		}
		summary, err := validateTxWitness(tx)
		if err != nil {
			return errors.Wrapf(err, "transaction %s", tx.TxHash)
		}
		aggregate.Contracts += summary.Contracts
		aggregate.Entries += summary.Entries
		aggregate.ProofNodes += summary.ProofNodes
		aggregate.ProofBytes += summary.ProofBytes
	}

	if result.Summary != nil && *result.Summary != aggregate {
		return errors.Errorf("block witness summary mismatch: expected %+v got %+v", aggregate, *result.Summary)
	}
	return nil
}

func validateTxWitness(tx txWitnessEnvelope) (witnessSummary, error) {
	var summary witnessSummary
	for _, witness := range tx.Witnesses {
		evmWitness, err := witness.toEVMWitness()
		if err != nil {
			return summary, err
		}
		addr := common.HexToAddress(witness.Address)
		if err := evm.VerifyContractStorageWitness(addr, evmWitness); err != nil {
			return summary, errors.Wrapf(err, "contract %s", witness.Address)
		}
		summary.Contracts++
		summary.Entries += uint64(len(witness.Entries))
		summary.ProofNodes += uint64(len(witness.ProofNodes))
		for _, node := range witness.ProofNodes {
			summary.ProofBytes += uint64(len(common.FromHex(node)))
		}
	}
	if tx.Contracts != summary.Contracts || tx.Entries != summary.Entries || tx.ProofNodes != summary.ProofNodes || tx.ProofBytes != summary.ProofBytes {
		return summary, errors.Errorf("tx witness summary mismatch: expected %+v got {Contracts:%d Entries:%d ProofNodes:%d ProofBytes:%d}",
			summary, tx.Contracts, tx.Entries, tx.ProofNodes, tx.ProofBytes)
	}
	return summary, nil
}

func (w contractWitness) toEVMWitness() (*evm.ContractStorageWitness, error) {
	rootBytes := common.FromHex(w.StorageRoot)
	if len(rootBytes) != len(common.Hash{}) {
		return nil, errors.Errorf("invalid storage root length for %s", w.Address)
	}
	var storageRoot common.Hash
	copy(storageRoot[:], rootBytes)

	out := &evm.ContractStorageWitness{
		ProofNodes: make([][]byte, 0, len(w.ProofNodes)),
		Entries:    make([]evm.ContractStorageWitnessEntry, 0, len(w.Entries)),
	}
	copy(out.StorageRoot[:], storageRoot[:])
	for _, entry := range w.Entries {
		keyBytes := common.FromHex(entry.Key)
		if len(keyBytes) != len(common.Hash{}) {
			return nil, errors.Errorf("invalid storage key length for %s", entry.Key)
		}
		var key common.Hash
		copy(key[:], keyBytes)

		witnessEntry := evm.ContractStorageWitnessEntry{}
		copy(witnessEntry.Key[:], key[:])
		if entry.Value != "" {
			witnessEntry.Value = common.FromHex(entry.Value)
		}
		out.Entries = append(out.Entries, witnessEntry)
	}
	for _, node := range w.ProofNodes {
		out.ProofNodes = append(out.ProofNodes, common.FromHex(node))
	}
	return out, nil
}

func initialHeight(ctx context.Context, client *rpcClient, requested uint64, once bool) (uint64, error) {
	if requested > 0 {
		return requested, nil
	}
	latest, err := client.blockNumber(ctx)
	if err != nil {
		return 0, err
	}
	if once {
		if latest == 0 {
			return 0, errors.New("no block available to verify")
		}
		return latest, nil
	}
	return latest + 1, nil
}

func (c *rpcClient) blockNumber(ctx context.Context) (uint64, error) {
	var out string
	if err := c.call(ctx, "eth_blockNumber", []any{}, &out); err != nil {
		return 0, err
	}
	return decodeHexUint64(out)
}

func (c *rpcClient) blockByNumber(ctx context.Context, height uint64) (*blockResult, error) {
	var out blockResult
	if err := c.call(ctx, "eth_getBlockByNumber", []any{encodeHeight(height), false}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *rpcClient) blockWitnessByNumber(ctx context.Context, height uint64) (*blockWitnessResult, error) {
	var out blockWitnessResult
	if err := c.call(ctx, "debug_traceBlockWitnessByNumber", []any{encodeHeight(height), map[string]any{}}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *rpcClient) call(ctx context.Context, method string, params any, out any) error {
	reqBody, err := json.Marshal(rpcRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  method,
		Params:  params,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var rpcResp rpcResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return err
	}
	if rpcResp.Error != nil {
		return errors.Errorf("rpc %s failed: (%d) %s", method, rpcResp.Error.Code, rpcResp.Error.Message)
	}
	if len(rpcResp.Result) == 0 || bytes.Equal(rpcResp.Result, []byte("null")) {
		return errors.Errorf("rpc %s returned empty result", method)
	}
	return json.Unmarshal(rpcResp.Result, out)
}

func decodeHexUint64(v string) (uint64, error) {
	var out hexutil.Uint64
	if err := out.UnmarshalText([]byte(v)); err != nil {
		return 0, err
	}
	return uint64(out), nil
}

func encodeHeight(height uint64) string {
	return hexutil.EncodeUint64(height)
}

func fatalf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
