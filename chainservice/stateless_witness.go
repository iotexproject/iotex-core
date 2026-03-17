package chainservice

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action/protocol/execution/evm"
	"github.com/iotexproject/iotex-core/v2/blockchain/witness"
)

type (
	statelessWitnessRPCClient struct {
		endpoint string
		client   *http.Client
	}

	statelessWitnessRPCRequest struct {
		JSONRPC string `json:"jsonrpc"`
		ID      int    `json:"id"`
		Method  string `json:"method"`
		Params  any    `json:"params"`
	}

	statelessWitnessRPCError struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}

	statelessWitnessRPCResponse struct {
		Result json.RawMessage           `json:"result"`
		Error  *statelessWitnessRPCError `json:"error"`
	}
)

var _emptyBlockWitnessResult = json.RawMessage(`{"transactions":[]}`)

func newStatelessWitnessRPCClient(endpoint string) *statelessWitnessRPCClient {
	return &statelessWitnessRPCClient{
		endpoint: strings.TrimRight(endpoint, "/"),
		client: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
}

func (c *statelessWitnessRPCClient) blockWitnessByHash(ctx context.Context, blockHash hash.Hash256) (evm.StatelessValidationContext, error) {
	raw, err := c.blockWitnessByHashRaw(ctx, blockHash)
	if err != nil {
		return evm.StatelessValidationContext{}, err
	}
	return witness.ParseValidationContext(raw)
}

func (c *statelessWitnessRPCClient) blockWitnessByHashRaw(ctx context.Context, blockHash hash.Hash256) (json.RawMessage, error) {
	var result json.RawMessage
	if err := c.call(ctx, "debug_getBlockWitnessByHash", []any{common.BytesToHash(blockHash[:]).Hex()}, &result); err != nil {
		if strings.Contains(err.Error(), "not exist in DB") {
			return append(json.RawMessage(nil), _emptyBlockWitnessResult...), nil
		}
		return nil, err
	}
	return append(json.RawMessage(nil), result...), nil
}

func (c *statelessWitnessRPCClient) call(ctx context.Context, method string, params any, out any) error {
	reqBody, err := json.Marshal(statelessWitnessRPCRequest{
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

	var rpcResp statelessWitnessRPCResponse
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
