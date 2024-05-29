package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strconv"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/pkg/errors"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/go-pkgs/util"

	"github.com/iotexproject/iotex-core/pkg/log"
)

type (
	proxyHandler struct {
		proxys map[uint64]*httputil.ReverseProxy
		shards []ProxyShard
		cs     CoreService
	}
)

func newProxyHandler(shards []ProxyShard, cs CoreService, endpoint string) (*proxyHandler, error) {
	h := &proxyHandler{
		proxys: make(map[uint64]*httputil.ReverseProxy),
		shards: shards,
		cs:     cs,
	}
	if len(shards) == 0 {
		return nil, errors.New("no shards provided")
	}
	for i := range shards {
		shard := &shards[i]
		// use the endpoint from the config if it's not empty
		if endpoint != "" {
			shard.Endpoint = endpoint
		}
		targetURL, err := url.Parse(shard.Endpoint)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse endpoint %s", shard.Endpoint)
		}
		proxy := httputil.NewSingleHostReverseProxy(targetURL)
		h.proxys[shard.ID] = proxy
	}
	return h, nil
}

func (handler *proxyHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Method != "POST" {
		handler.proxyOfShard(handler.shardByHeight(rpc.LatestBlockNumber).ID).ServeHTTP(w, req)
		return
	}
	bodyBytes, err := io.ReadAll(req.Body)
	if err != nil {
		handler.error(w, err)
		return
	}
	web3Reqs, err := parseWeb3Reqs(io.NopCloser(bytes.NewBuffer(bodyBytes)))
	if err != nil {
		handler.error(w, err)
		return
	}
	req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	if !web3Reqs.IsArray() {
		blkNum, err := handler.parseWeb3Height(&web3Reqs)
		if err != nil {
			log.L().Error("failed to parse block number", zap.Error(err))
			blkNum = rpc.LatestBlockNumber
		}
		shard := handler.shardByHeight(blkNum)
		appendShardToRequest(req, shard)
		log.L().Info("forwarding request to shard", zap.Uint64("shard", shard.ID), zap.String("request height", blkNum.String()), zap.String("endpoint", shard.Endpoint), zap.String("url", req.URL.RequestURI()))
		proxy := handler.proxyOfShard(shard.ID)
		proxy.ServeHTTP(w, req)
	} else {
		type shardReq struct {
			index  int
			blkNum rpc.BlockNumber
			req    *gjson.Result
		}
		reqs := web3Reqs.Array()
		groupedReqs := make(map[uint64][]*shardReq)
		for i := range reqs {
			blkNum, err := handler.parseWeb3Height(&reqs[i])
			if err != nil {
				log.L().Error("failed to parse block number", zap.Error(err))
				blkNum = rpc.LatestBlockNumber
			}
			shard := handler.shardByHeight(blkNum)
			groupedReqs[shard.ID] = append(groupedReqs[shard.ID], &shardReq{
				index:  i,
				req:    &reqs[i],
				blkNum: blkNum,
			})
		}
		bodies := make([][]byte, len(reqs))
		for shardID, reqs := range groupedReqs {
			shard := &handler.shards[shardID]
			groupBodies := make([][]byte, len(reqs))
			for i := range reqs {
				groupBodies[i] = []byte(reqs[i].req.Raw)
			}
			body := append([]byte("["), bytes.Join(groupBodies, []byte(","))...)
			body = append(body, []byte("]")...)
			req.Body = io.NopCloser(bytes.NewBuffer(body))
			req.ContentLength = int64(len(body))
			appendShardToRequest(req, shard)
			writer := httptest.NewRecorder()
			log.L().Info("forwarding batch request to shard", zap.Uint64("shard", shard.ID), zap.Int("batch request size", len(reqs)), zap.String("endpoint", shard.Endpoint), zap.String("url", req.URL.RequestURI()))
			handler.proxyOfShard(shard.ID).ServeHTTP(writer, req)
			subResps := gjson.ParseBytes(writer.Body.Bytes()).Array()
			for i, subResp := range subResps {
				bodies[reqs[i].index] = []byte(subResp.Raw)
			}
		}
		// write all bodies to response
		allBody := append([]byte("["), bytes.Join(bodies, []byte(","))...)
		allBody = append(allBody, []byte("]")...)
		_, err = w.Write(allBody)
		if err != nil {
			log.L().Error("failed to write response", zap.Error(err))
		}
	}
}

func (handler *proxyHandler) error(w http.ResponseWriter, err error) {
	log.L().Error("failed to get request body", zap.Error(err))
	raw, err := json.Marshal(&web3Response{err: err})
	if err != nil {
		log.L().Error("failed to marshal error response", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		w.Write(raw)
	}
}

func (handler *proxyHandler) shardByHeight(height rpc.BlockNumber) *ProxyShard {
	shards := handler.shards
	// if height is latest, return the last shard
	if height.Int64() < 0 {
		return &shards[len(shards)-1]
	}
	// find the shard by historical height
	for _, shard := range shards {
		if uint64(height.Int64()) >= shard.StartHeight && uint64(height.Int64()) < shard.EndHeight {
			return &shard
		}
	}
	// if not found, return the last shard
	return &shards[len(shards)-1]
}

func (handler *proxyHandler) parseWeb3Height(web3Req *gjson.Result) (rpc.BlockNumber, error) {
	var (
		method = web3Req.Get("method").Value()
		blkNum rpc.BlockNumber
		err    error
	)
	switch method {
	case "eth_getBalance", "eth_getCode", "eth_getTransactionCount", "eth_call", "eth_estimateGas", "debug_traceCall":
		blkParam := web3Req.Get("params.1")
		blkNum, err = parseBlockNumber(&blkParam)
	case "eth_getStorageAt":
		blkParam := web3Req.Get("params.2")
		blkNum, err = parseBlockNumber(&blkParam)
	case "debug_traceBlockByNumber":
		blkParam := web3Req.Get("params.0")
		blkNum, err = parseBlockNumber(&blkParam)
	case "debug_traceBlockByHash":
		blkHash := web3Req.Get("params.0")
		blk, err := handler.cs.BlockByHash(util.Remove0xPrefix(blkHash.String()))
		if err != nil {
			return 0, err
		}
		blkNum = rpc.BlockNumber(blk.Block.Height())
	case "debug_traceTransaction":
		txHash := web3Req.Get("params.0")
		actHash, err := hash.HexStringToHash256(util.Remove0xPrefix(txHash.String()))
		if err != nil {
			return 0, err
		}
		_, blk, _, err := handler.cs.ActionByActionHash(actHash)
		if err != nil {
			return 0, err
		}
		blkNum = rpc.BlockNumber(blk.Height())
	default:
		blkNum = rpc.LatestBlockNumber
	}
	return blkNum, err
}

func (handler *proxyHandler) proxyOfShard(shardID uint64) *httputil.ReverseProxy {
	return handler.proxys[shardID]
}

func appendShardToRequest(r *http.Request, shard *ProxyShard) {
	query := r.URL.Query()
	query.Set("shard", strconv.FormatUint(shard.ID, 10))
	r.URL.RawQuery = query.Encode()
}
