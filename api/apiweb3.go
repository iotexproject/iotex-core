package api

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"

	"google.golang.org/grpc/status"
)

type (
	web3Req struct {
		Jsonrpc string      `json:"jsonrpc"`
		Id      int         `json:"id"`
		Method  string      `json:"method"`
		Params  interface{} `json:"params"`
	}

	web3Resp struct {
		Jsonrpc string      `json:"jsonrpc"`
		Id      int         `json:"id"`
		Result  interface{} `json:"result,omitempty"`
		Error   Web3Err     `json:"error,omitempty"`
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
	case "GET":
		w.Write([]byte{'2', '\n'})
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (api *Server) handlePOSTReq(w http.ResponseWriter, req *http.Request) {

	web3Reqs := getWeb3Reqs(req)
	var web3Resp []Web3Resp

	for _, web3Req := range web3Reqs {
		if _, ok := apiMap[web3Req.Method]; !ok {
			panic(ok) // not exist
		}

		res, err := apiMap[web3Req.Method](api, web3Req.Params)
		var resp Web3Resp
		if err != nil {
			s, ok := status.FromError(err)
			if !ok {
				panic(err)
			}
			resp = Web3Resp{
				Jsonrpc: "2.0",
				Id:      web3Req.Id,
				Error: Web3Err{
					Code:    int(s.Code()),
					Message: s.Message(),
				},
			}
		} else {
			resp = Web3Resp{
				Jsonrpc: "2.0",
				Id:      web3Req.Id,
				Result:  res,
			}
		}
		web3Resp = append(web3Resp, resp)
	}

	// write responses
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	if len(web3Resp) > 1 {
		if err := json.NewEncoder(w).Encode(web3Resp); err != nil {
			panic(err)
		}
	} else {
		if err := json.NewEncoder(w).Encode(web3Resp[0]); err != nil {
			panic(err)
		}
	}

}

func isJSONArray(data []byte) bool {
	data = bytes.TrimLeft(data, " \t\r\n")
	return len(data) > 0 && data[0] == '['
}

func getWeb3Reqs(req *http.Request) []Web3Req {
	var web3Reqs []Web3Req
	data, err := ioutil.ReadAll(req.Body)
	if err != nil {
		panic(err)
	}
	if isJSONArray(data) {
		err := json.Unmarshal(data, &web3Reqs)
		if err != nil {
			panic(err)
		}
	} else {
		var web3Req Web3Req
		err := json.Unmarshal(data, &web3Req)
		if err != nil {
			panic(err)
		}
		web3Reqs = append(web3Reqs, web3Req)
	}
	return web3Reqs
}
