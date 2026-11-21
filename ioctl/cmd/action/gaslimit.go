// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/output"
)

// _maxTxGasLimit is the per-transaction gas limit cap of EIP-7825 (2^24).
// An estimated gas limit above it is reported instead of being sent, since the
// transaction would be rejected once the cap is active.
const _maxTxGasLimit = uint64(1 << 24)

// Gas estimators that query the node. They are variables so tests can stub them.
var (
	_estimateExecutionGas    = fixGasLimit
	_estimateMigrateStakeGas = migrateGasLimit
	_estimateSetCodeTxGas    = nodeEstimateSetCodeTxGas
)

// resolveGasLimit returns the gas limit to send elp with. A non-zero gas limit,
// i.e. one given by --gas-limit, is used as is. A zero gas limit is estimated:
// native actions use their intrinsic gas, while contract executions and stake
// migrations, which run the EVM, are estimated by the node.
func resolveGasLimit(caller string, elp action.Envelope) (uint64, error) {
	if gas := elp.Gas(); gas != 0 {
		return gas, nil
	}
	var (
		gas uint64
		err error
	)
	switch act := elp.Action().(type) {
	case *action.Execution:
		gas, err = _estimateExecutionGas(caller, act)
	case *action.MigrateStake:
		gas, err = _estimateMigrateStakeGas(caller, act)
	default:
		gas, err = elp.IntrinsicGas()
	}
	if err != nil {
		return 0, output.NewError(0, "failed to estimate gas limit, use --gas-limit to set it explicitly", err)
	}
	if err := checkEstimatedGas(gas); err != nil {
		return 0, err
	}
	return gas, nil
}

// checkEstimatedGas rejects an estimated gas limit above the per-transaction cap.
func checkEstimatedGas(gas uint64) error {
	if gas > _maxTxGasLimit {
		return output.NewError(output.ValidationError, fmt.Sprintf(
			"estimated gas %d exceeds the per-transaction gas limit cap %d (EIP-7825); "+
				"use --gas-limit to set the gas limit explicitly", gas, _maxTxGasLimit), nil)
	}
	return nil
}

// resolveSetCodeTxGasLimit is resolveGasLimit for an EIP-7702 SetCodeTx: the
// gas is estimated by the node's Web3 eth_estimateGas, which applies the
// authorization list during the simulation.
func resolveSetCodeTxGasLimit(gasLimit uint64, from, to common.Address, value *big.Int, data []byte, auths []types.SetCodeAuthorization) (uint64, error) {
	if gasLimit != 0 {
		return gasLimit, nil
	}
	gas, err := _estimateSetCodeTxGas(from, to, value, data, auths)
	if err != nil {
		return 0, output.NewError(0, "failed to estimate gas limit, use --gas-limit to set it explicitly", err)
	}
	if err := checkEstimatedGas(gas); err != nil {
		return 0, err
	}
	return gas, nil
}

// nodeEstimateSetCodeTxGas calls eth_estimateGas on the Web3 endpoint of the
// configured node.
func nodeEstimateSetCodeTxGas(from, to common.Address, value *big.Int, data []byte, auths []types.SetCodeAuthorization) (uint64, error) {
	secure := config.ReadConfig.SecureConnect && !config.Insecure
	url := deriveWeb3Endpoint(config.ReadConfig.Endpoint, secure)
	if url == "" {
		return 0, fmt.Errorf("cannot derive a web3 endpoint from %q", config.ReadConfig.Endpoint)
	}
	return fetchEthEstimateGas(url, from, to, value, data, auths)
}

// fetchEthEstimateGas calls eth_estimateGas on a Web3 JSON-RPC endpoint for a
// SetCodeTx-shaped call and parses the hex-encoded result.
func fetchEthEstimateGas(url string, from, to common.Address, value *big.Int, data []byte, auths []types.SetCodeAuthorization) (uint64, error) {
	if value == nil {
		value = new(big.Int)
	}
	call := struct {
		From              common.Address               `json:"from"`
		To                common.Address               `json:"to"`
		Value             *hexutil.Big                 `json:"value"`
		Data              hexutil.Bytes                `json:"data"`
		AuthorizationList []types.SetCodeAuthorization `json:"authorizationList,omitempty"`
	}{from, to, (*hexutil.Big)(value), data, auths}
	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"method":  "eth_estimateGas",
		"params":  []any{call},
		"id":      1,
	})
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("web3 eth_estimateGas returned HTTP %d", resp.StatusCode)
	}

	var out struct {
		Result string `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return 0, err
	}
	if out.Error != nil {
		return 0, fmt.Errorf("web3 eth_estimateGas error: %s", out.Error.Message)
	}
	hexStr := strings.TrimPrefix(strings.TrimSpace(out.Result), "0x")
	if hexStr == "" {
		return 0, fmt.Errorf("web3 eth_estimateGas returned empty result")
	}
	gas, err := strconv.ParseUint(hexStr, 16, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse eth_estimateGas result %q: %w", out.Result, err)
	}
	return gas, nil
}
