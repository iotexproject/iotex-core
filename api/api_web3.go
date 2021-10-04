package api

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/iotexproject/iotex-core/pkg/log"
	"go.uber.org/zap"
	"google.golang.org/grpc/status"
)

type (
	web3Req struct {
		Jsonrpc string      `json:"jsonrpc"`
		ID      int         `json:"id"`
		Method  string      `json:"method"`
		Params  interface{} `json:"params"`
	}

	web3Resp struct {
		Jsonrpc string      `json:"jsonrpc"`
		ID      int         `json:"id"`
		Result  interface{} `json:"result,omitempty"`
		Error   *web3Err    `json:"error,omitempty"`
	}

	web3Err struct {
		Code    int    `json:"code,omitempty"`
		Message string `json:"message,omitempty"`
	}
)

func (api *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case "POST":
		api.handlePOSTReq(w, req)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (api *Server) handlePOSTReq(w http.ResponseWriter, req *http.Request) {

	web3Reqs, err := getWeb3Reqs(req)
	if err != nil {
		log.L().Error("Failed to parse web3 requests.", zap.Error(err))
		return
	}

	var web3Resps []web3Resp
	for _, web3Req := range web3Reqs {
		if _, ok := apiMap[web3Req.Method]; !ok {
			log.L().Warn("unsupported web3 method.", zap.String("method", web3Req.Method))
			return
		}

		res, err := apiMap[web3Req.Method](api, web3Req.Params)
		var resp web3Resp
		if err != nil {
			s, ok := status.FromError(err)
			var errCode int
			var errMsg string
			if ok {
				errCode, errMsg = int(s.Code()), s.Message()
			} else {
				errCode, errMsg = -1, err.Error()
			}
			resp = web3Resp{
				Jsonrpc: "2.0",
				ID:      web3Req.ID,
				Error: &web3Err{
					Code:    errCode,
					Message: errMsg,
				},
			}
		} else {
			resp = web3Resp{
				Jsonrpc: "2.0",
				ID:      web3Req.ID,
				Result:  res,
			}
		}
		web3Resps = append(web3Resps, resp)
	}

	// write responses
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	if len(web3Resps) > 1 {
		if err := json.NewEncoder(w).Encode(web3Resps); err != nil {
			log.L().Warn("fail to respond request.")
			return
		}
	} else {
		if err := json.NewEncoder(w).Encode(web3Resps[0]); err != nil {
			log.L().Warn("fail to respond request.")
			return
		}
	}

}

func isJSONArray(data []byte) bool {
	data = bytes.TrimLeft(data, " \t\r\n")
	return len(data) > 0 && data[0] == '['
}

func getWeb3Reqs(req *http.Request) ([]web3Req, error) {
	var web3Reqs []web3Req
	data, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	if isJSONArray(data) {
		err := json.Unmarshal(data, &web3Reqs)
		if err != nil {
			return nil, err
		}
	} else {
		var web3Req web3Req
		err := json.Unmarshal(data, &web3Req)
		if err != nil {
			return nil, err
		}
		web3Reqs = append(web3Reqs, web3Req)
	}
	return web3Reqs, nil
}
