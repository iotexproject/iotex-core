package api

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/pkg/errors"
	"google.golang.org/grpc/status"
)

const (
	contentType = "application/json"
)

type (
	web3Req struct {
		Jsonrpc string      `json:"jsonrpc,omitempty"`
		ID      *int        `json:"id"`
		Method  *string     `json:"method"`
		Params  interface{} `json:"params"`
	}

	web3Resp struct {
		Jsonrpc string      `json:"jsonrpc"`
		ID      int         `json:"id"`
		Result  interface{} `json:"result,omitempty"`
		Error   *web3Err    `json:"error,omitempty"`
	}

	web3Err struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
)

func (api *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case "POST":
		api.handlePOSTReq(w, req)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (api *Server) handlePOSTReq(w http.ResponseWriter, req *http.Request) {
	web3Reqs, err := parseWeb3Reqs(req)
	var web3Resps []web3Resp
	var httpResp interface{}
	if err != nil {
		err := errors.Wrap(err, "failed to parse web3 requests.")
		httpResp = packAPIResult(nil, err, 0)
		goto Respond
	}

	for _, web3Req := range web3Reqs {
		if _, ok := apiMap[*web3Req.Method]; !ok {
			err := errors.Wrapf(errors.New("web3 method not found"), "method: %s\n", *web3Req.Method)
			httpResp = packAPIResult(nil, err, 0)
			goto Respond
		}

		res, err := apiMap[*web3Req.Method](api, web3Req.Params)
		web3Resps = append(web3Resps, packAPIResult(res, err, *web3Req.ID))
	}

	switch {
	case len(web3Resps) > 1:
		httpResp = web3Resps
	case len(web3Resps) == 1:
		httpResp = web3Resps[0]
	}

	// write responses
Respond:
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(httpResp); err != nil {
		log.L().Warn("fail to respond request.")
	}
}

func parseWeb3Reqs(req *http.Request) ([]web3Req, error) {
	if req.Header.Get("Content-type") != "application/json" {
		return nil, errors.New("content-type is not application/json")
	}

	var web3Reqs []web3Req
	data, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	if d := bytes.TrimLeft(data, " \t\r\n"); len(d) > 0 && (d[0] != '[' || d[len(d)-1] != ']') {
		data = []byte("[" + string(d) + "]")
	}
	err = json.Unmarshal(data, &web3Reqs)
	if err != nil {
		return nil, err
	}
	for _, v := range web3Reqs {
		if err := v.CheckRequiredField(); err != nil {
			return nil, err
		}
	}
	return web3Reqs, nil
}

func (req *web3Req) CheckRequiredField() error {
	if req.ID == nil || req.Method == nil || req.Params == nil {
		return errors.New("request field is incomplete")
	}
	return nil
}

// error code: https://eth.wiki/json-rpc/json-rpc-error-codes-improvement-proposal
func packAPIResult(res interface{}, err error, id int) web3Resp {
	if err != nil {
		s, ok := status.FromError(err)
		var errCode int
		var errMsg string
		if ok {
			errCode, errMsg = int(s.Code()), s.Message()
		} else {
			errCode, errMsg = -32603, err.Error()
		}
		return web3Resp{
			Jsonrpc: "2.0",
			ID:      id,
			Error: &web3Err{
				Code:    errCode,
				Message: errMsg,
			},
		}
	}
	return web3Resp{
		Jsonrpc: "2.0",
		ID:      id,
		Result:  res,
	}
}
