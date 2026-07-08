// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/output"
)

// The EVM chain id (a.k.a. evmNetworkID) is required to sign an eth-style tx and
// differs from the iotex chain id. The authoritative source is the node itself:
// we query its Web3 JSON-RPC `eth_chainId`. The hardcoded mapping below is kept
// ONLY as an offline fallback for when the node cannot be reached or does not
// expose a Web3 endpoint (e.g. an old node, or a gRPC-only deployment).
var _evmChainIDFallback = map[uint32]uint64{
	1: 4689, // mainnet
	2: 4690, // testnet
	3: 4691, // nightly / local-dev
}

// _web3Port is the default Web3 JSON-RPC HTTP port of an iotex node (see
// api/config.go: HTTPPort). Used when deriving a Web3 URL for a self-hosted node.
const _web3Port = 15014

// resolveEVMChainID picks the EVM chain id used to sign an eth-style tx.
// Precedence:
//  1. explicit --evm-chain-id flag (ultimate escape hatch, never overridden)
//  2. value reported by the node via Web3 eth_chainId (authoritative)
//  3. offline fallback mapping keyed by the iotex chain id
//  4. error, when none of the above yields a value
//
// fetchFromNode is injected for testability; production passes nodeEVMChainID.
// A node value of 0, or any fetch error, silently degrades to the fallback so a
// gRPC-only / offline node still works for the common networks.
func resolveEVMChainID(iotexChainID uint32, fetchFromNode func() (uint64, error)) (uint64, error) {
	if v := _evmChainIDFlag.Value().(uint64); v != 0 {
		return v, nil
	}
	if fetchFromNode != nil {
		if v, err := fetchFromNode(); err == nil && v != 0 {
			return v, nil
		}
	}
	if v, ok := _evmChainIDFallback[iotexChainID]; ok {
		return v, nil
	}
	return 0, output.NewError(output.InputError,
		fmt.Sprintf("could not determine EVM chain id for iotex chain id %d; set --evm-chain-id explicitly", iotexChainID), nil)
}

// nodeEVMChainID queries the configured node's Web3 JSON-RPC eth_chainId and
// returns the evmNetworkID. The Web3 URL is derived from the gRPC endpoint that
// ioctl is already configured to talk to.
func nodeEVMChainID() (uint64, error) {
	// Mirror the gRPC dial's secure decision: the one-shot --insecure flag
	// (config.Insecure) overrides the persisted secureConnect setting.
	secure := config.ReadConfig.SecureConnect && !config.Insecure
	url := deriveWeb3Endpoint(config.ReadConfig.Endpoint, secure)
	if url == "" {
		return 0, fmt.Errorf("cannot derive a web3 endpoint from %q", config.ReadConfig.Endpoint)
	}
	return fetchEthChainID(url)
}

// deriveWeb3Endpoint best-effort derives the node's Web3 JSON-RPC URL from the
// configured gRPC endpoint. Public IoTeX gRPC hosts are mapped to their
// babel-api Web3 hosts; any other host:port endpoint is treated as a
// self-hosted node whose Web3 JSON-RPC is served on the default web3 port.
// Returns "" when no sensible guess is possible.
func deriveWeb3Endpoint(grpcEndpoint string, secure bool) string {
	grpcEndpoint = strings.TrimSpace(grpcEndpoint)
	if grpcEndpoint == "" {
		return ""
	}
	host := grpcEndpoint
	if h, _, err := net.SplitHostPort(grpcEndpoint); err == nil {
		host = h
	}
	switch host {
	case "api.iotex.one", "api.mainnet.iotex.one", "api.mainnet.iotex.io":
		return "https://babel-api.mainnet.iotex.io"
	case "api.testnet.iotex.one", "api.testnet.iotex.io":
		return "https://babel-api.testnet.iotex.io"
	}
	scheme := "http"
	if secure {
		scheme = "https"
	}
	// net.JoinHostPort brackets IPv6 hosts (e.g. "[::1]:15014") correctly.
	return fmt.Sprintf("%s://%s", scheme, net.JoinHostPort(host, strconv.Itoa(_web3Port)))
}

// fetchEthChainID calls eth_chainId on a Web3 JSON-RPC endpoint and parses the
// hex-encoded result into a uint64.
func fetchEthChainID(url string) (uint64, error) {
	const body = `{"jsonrpc":"2.0","method":"eth_chainId","params":[],"id":1}`
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("web3 eth_chainId returned HTTP %d", resp.StatusCode)
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
		return 0, fmt.Errorf("web3 eth_chainId error: %s", out.Error.Message)
	}
	hexStr := strings.TrimPrefix(strings.TrimSpace(out.Result), "0x")
	if hexStr == "" {
		return 0, fmt.Errorf("web3 eth_chainId returned empty result")
	}
	id, err := strconv.ParseUint(hexStr, 16, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse eth_chainId result %q: %w", out.Result, err)
	}
	return id, nil
}
