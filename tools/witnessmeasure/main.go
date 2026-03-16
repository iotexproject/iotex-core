// witnessmeasure measures contract storage witness size and verification time
// across a range of blocks fetched from a node that exposes
// debug_traceBlockWitnessByNumber.
//
// Usage:
//
//	witnessmeasure -rpc <endpoint> -heights 28476059,28583678,28009641
//	witnessmeasure -rpc <endpoint> -from 28476059 -to 28476069
//	witnessmeasure -rpc <endpoint> -csv samples.csv   # read heights from CSV file
package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action/protocol/execution/evm"
	"github.com/iotexproject/iotex-core/v2/blockchain/witness"
)

// ---- RPC client ----

type rpcClient struct {
	endpoint string
	client   *http.Client
}

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *rpcError       `json:"error"`
}

func newClient(endpoint string) *rpcClient {
	return &rpcClient{
		endpoint: endpoint,
		client:   &http.Client{Timeout: 60 * time.Second},
	}
}

func (c *rpcClient) call(ctx context.Context, method string, params any, out any) error {
	body, err := json.Marshal(rpcRequest{JSONRPC: "2.0", ID: 1, Method: method, Params: params})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var rpcResp rpcResponse
	if err := json.Unmarshal(raw, &rpcResp); err != nil {
		return err
	}
	if rpcResp.Error != nil {
		return errors.Errorf("rpc error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}
	return json.Unmarshal(rpcResp.Result, out)
}

// blockResult is a minimal eth_getBlockByNumber response.
type blockResult struct {
	Number       string   `json:"number"`
	Hash         string   `json:"hash"`
	Transactions []string `json:"transactions"`
}

func (c *rpcClient) blockByNumber(ctx context.Context, height uint64) (*blockResult, error) {
	var out blockResult
	err := c.call(ctx, "eth_getBlockByNumber", []any{fmt.Sprintf("0x%x", height), false}, &out)
	return &out, err
}

func (c *rpcClient) blockWitnessByNumber(ctx context.Context, height uint64) (json.RawMessage, *witness.BlockResult, error) {
	var raw json.RawMessage
	if err := c.call(ctx, "debug_traceBlockWitnessByNumber", []any{fmt.Sprintf("0x%x", height), map[string]any{}}, &raw); err != nil {
		return nil, nil, err
	}
	var result witness.BlockResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, nil, err
	}
	return raw, &result, nil
}

// ---- Measurement ----

type blockMeasurement struct {
	Height       uint64
	TotalTxs     int
	ExecTxs      int // txs that have at least one contract witness
	Contracts    uint64
	Entries      uint64
	ProofNodes   uint64
	ProofBytes   uint64
	WitnessBytes int // raw JSON size
	VerifyNs     int64
	VerifyErr    string
}

func measureBlock(ctx context.Context, client *rpcClient, height uint64) (blockMeasurement, error) {
	m := blockMeasurement{Height: height}

	blk, err := client.blockByNumber(ctx, height)
	if err != nil {
		return m, errors.Wrapf(err, "fetch block %d", height)
	}
	m.TotalTxs = len(blk.Transactions)

	raw, result, err := client.blockWitnessByNumber(ctx, height)
	if err != nil {
		return m, errors.Wrapf(err, "fetch witness %d", height)
	}
	m.WitnessBytes = len(raw)

	if result.Summary != nil {
		m.Contracts = result.Summary.Contracts
		m.Entries = result.Summary.Entries
		m.ProofNodes = result.Summary.ProofNodes
		m.ProofBytes = result.Summary.ProofBytes
	}
	for _, tx := range result.Transactions {
		if len(tx.Witnesses) > 0 {
			m.ExecTxs++
		}
	}

	// Time verification
	start := time.Now()
	var verifyErr string
	for _, tx := range result.Transactions {
		for _, cw := range tx.Witnesses {
			evmWitness, err := cw.ToEVMWitness()
			if err != nil {
				verifyErr = err.Error()
				break
			}
			if err := evm.VerifyContractStorageWitness(common.HexToAddress(cw.Address), evmWitness); err != nil {
				verifyErr = err.Error()
				break
			}
		}
		if verifyErr != "" {
			break
		}
	}
	m.VerifyNs = time.Since(start).Nanoseconds()
	m.VerifyErr = verifyErr

	return m, nil
}

// ---- CSV / table output ----

func printCSV(w io.Writer, measurements []blockMeasurement) {
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{
		"height", "total_txs", "exec_txs",
		"contracts", "entries", "proof_nodes", "proof_bytes", "witness_json_bytes",
		"verify_us", "verify_us_per_contract", "error",
	})
	for _, m := range measurements {
		verifyUs := m.VerifyNs / 1000
		var verifyPerContract int64
		if m.Contracts > 0 {
			verifyPerContract = verifyUs / int64(m.Contracts)
		}
		_ = cw.Write([]string{
			strconv.FormatUint(m.Height, 10),
			strconv.Itoa(m.TotalTxs),
			strconv.Itoa(m.ExecTxs),
			strconv.FormatUint(m.Contracts, 10),
			strconv.FormatUint(m.Entries, 10),
			strconv.FormatUint(m.ProofNodes, 10),
			strconv.FormatUint(m.ProofBytes, 10),
			strconv.Itoa(m.WitnessBytes),
			strconv.FormatInt(verifyUs, 10),
			strconv.FormatInt(verifyPerContract, 10),
			m.VerifyErr,
		})
	}
	cw.Flush()
}

func printTable(w io.Writer, measurements []blockMeasurement) {
	fmt.Fprintf(w, "%-12s %9s %9s %10s %8s %11s %11s %16s %10s %s\n",
		"height", "total_txs", "exec_txs",
		"contracts", "entries", "proof_nodes", "proof_bytes", "witness_json_B",
		"verify_ms", "status")
	fmt.Fprintf(w, "%s\n", strings.Repeat("-", 110))
	for _, m := range measurements {
		status := "OK"
		if m.VerifyErr != "" {
			if len(m.VerifyErr) > 30 {
				status = m.VerifyErr[:30] + "..."
			} else {
				status = m.VerifyErr
			}
		}
		verifyMs := float64(m.VerifyNs) / 1e6
		fmt.Fprintf(w, "%-12d %9d %9d %10d %8d %11d %11d %16d %10.2f %s\n",
			m.Height, m.TotalTxs, m.ExecTxs,
			m.Contracts, m.Entries, m.ProofNodes, m.ProofBytes, m.WitnessBytes,
			verifyMs, status)
	}
}

func printSummary(w io.Writer, measurements []blockMeasurement) {
	var ok []blockMeasurement
	for _, m := range measurements {
		if m.VerifyErr == "" && m.Contracts > 0 {
			ok = append(ok, m)
		}
	}
	if len(ok) == 0 {
		fmt.Fprintln(w, "\nNo successful measurements with contract witnesses.")
		return
	}

	sort.Slice(ok, func(i, j int) bool { return ok[i].ExecTxs < ok[j].ExecTxs })

	var totalBytes, totalProofBytes, totalNs int64
	var totalContracts, totalEntries, totalProofNodes uint64
	for _, m := range ok {
		totalBytes += int64(m.WitnessBytes)
		totalProofBytes += int64(m.ProofBytes)
		totalNs += m.VerifyNs
		totalContracts += m.Contracts
		totalEntries += m.Entries
		totalProofNodes += m.ProofNodes
	}
	n := int64(len(ok))

	fmt.Fprintf(w, "\n=== Summary (%d blocks with witnesses) ===\n", n)
	fmt.Fprintf(w, "Witness JSON bytes  avg=%d  total=%d\n", totalBytes/n, totalBytes)
	fmt.Fprintf(w, "Proof bytes         avg=%d  total=%d\n", totalProofBytes/n, totalProofBytes)
	fmt.Fprintf(w, "Contracts/block     avg=%.1f  total=%d\n", float64(totalContracts)/float64(n), totalContracts)
	fmt.Fprintf(w, "Entries/block       avg=%.1f  total=%d\n", float64(totalEntries)/float64(n), totalEntries)
	fmt.Fprintf(w, "ProofNodes/block    avg=%.1f  total=%d\n", float64(totalProofNodes)/float64(n), totalProofNodes)
	fmt.Fprintf(w, "Verify time         avg=%.2f ms  total=%.2f ms\n",
		float64(totalNs)/float64(n)/1e6, float64(totalNs)/1e6)
	if totalContracts > 0 {
		fmt.Fprintf(w, "Verify time/contract avg=%.3f ms\n", float64(totalNs)/float64(totalContracts)/1e6)
		fmt.Fprintf(w, "ProofBytes/contract  avg=%.0f\n", float64(totalProofBytes)/float64(totalContracts))
		fmt.Fprintf(w, "Entries/contract     avg=%.1f\n", float64(totalEntries)/float64(totalContracts))
	}

	// Estimate scaling
	if len(ok) >= 3 {
		fmt.Fprintf(w, "\n--- Scaling estimate ---\n")
		// simple linear regression: proofBytes ~ a + b * execTxs
		n32 := float64(len(ok))
		var sumX, sumY, sumXX, sumXY float64
		for _, m := range ok {
			x := float64(m.ExecTxs)
			y := float64(m.ProofBytes)
			sumX += x
			sumY += y
			sumXX += x * x
			sumXY += x * y
		}
		denom := n32*sumXX - sumX*sumX
		if denom != 0 {
			b := (n32*sumXY - sumX*sumY) / denom
			a := (sumY - b*sumX) / n32
			fmt.Fprintf(w, "  proof_bytes  ≈ %.0f + %.1f × exec_txs\n", a, b)
		}

		sumY = 0
		sumXY = 0
		for _, m := range ok {
			x := float64(m.ExecTxs)
			y := float64(m.VerifyNs) / 1e6
			sumY += y
			sumXY += x * y
		}
		denom2 := n32*sumXX - sumX*sumX
		if denom2 != 0 {
			b := (n32*sumXY - sumX*sumY) / denom2
			a := (sumY - b*sumX) / n32
			fmt.Fprintf(w, "  verify_ms    ≈ %.3f + %.4f × exec_txs\n", a, b)
		}
	}
}

// ---- main ----

func parseHeights(s string) []uint64 {
	parts := strings.Split(s, ",")
	var out []uint64
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		v, err := strconv.ParseUint(p, 10, 64)
		if err == nil {
			out = append(out, v)
		}
	}
	return out
}

func readCSVHeights(path string) ([]uint64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	rows, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	var out []uint64
	for _, row := range rows {
		if len(row) == 0 {
			continue
		}
		v, err := strconv.ParseUint(strings.TrimSpace(row[0]), 10, 64)
		if err == nil {
			out = append(out, v)
		}
	}
	return out, nil
}

func main() {
	rpc := flag.String("rpc", "https://archive-testnet.iotex.io", "JSON-RPC endpoint")
	heightsFlag := flag.String("heights", "", "comma-separated block heights")
	fromFlag := flag.Uint64("from", 0, "start height (inclusive)")
	toFlag := flag.Uint64("to", 0, "end height (inclusive)")
	csvIn := flag.String("csv", "", "CSV file with heights in column 0")
	csvOut := flag.String("out", "", "write CSV results to this file (default: stdout)")
	concurrency := flag.Int("concurrency", 3, "parallel RPC requests")
	flag.Parse()

	var heights []uint64
	if *heightsFlag != "" {
		heights = append(heights, parseHeights(*heightsFlag)...)
	}
	if *fromFlag > 0 && *toFlag >= *fromFlag {
		for h := *fromFlag; h <= *toFlag; h++ {
			heights = append(heights, h)
		}
	}
	if *csvIn != "" {
		more, err := readCSVHeights(*csvIn)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to read CSV: %v\n", err)
			os.Exit(1)
		}
		heights = append(heights, more...)
	}
	if len(heights) == 0 {
		fmt.Fprintln(os.Stderr, "no block heights specified; use -heights, -from/-to, or -csv")
		flag.Usage()
		os.Exit(1)
	}

	client := newClient(*rpc)
	ctx := context.Background()

	type result struct {
		idx int
		m   blockMeasurement
		err error
	}

	// Fan-out with bounded concurrency
	semaphore := make(chan struct{}, *concurrency)
	results := make([]result, len(heights))
	done := make(chan result, len(heights))

	for i, h := range heights {
		i, h := i, h
		semaphore <- struct{}{}
		go func() {
			defer func() { <-semaphore }()
			m, err := measureBlock(ctx, client, h)
			done <- result{idx: i, m: m, err: err}
		}()
	}

	for range heights {
		r := <-done
		results[r.idx] = r
	}

	// Print human-readable table to stderr always
	var measurements []blockMeasurement
	for _, r := range results {
		if r.err != nil {
			fmt.Fprintf(os.Stderr, "height %d: %v\n", heights[r.idx], r.err)
			m := blockMeasurement{Height: heights[r.idx], VerifyErr: r.err.Error()}
			measurements = append(measurements, m)
			continue
		}
		measurements = append(measurements, r.m)
	}

	// Sort by height for readability
	sort.Slice(measurements, func(i, j int) bool { return measurements[i].Height < measurements[j].Height })

	printTable(os.Stderr, measurements)
	printSummary(os.Stderr, measurements)

	// Write CSV
	var csvWriter io.Writer = os.Stdout
	if *csvOut != "" {
		f, err := os.Create(*csvOut)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to create output file: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		csvWriter = f
		fmt.Fprintf(os.Stderr, "\nCSV written to %s\n", *csvOut)
	}
	printCSV(csvWriter, measurements)
}
