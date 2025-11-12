package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/semaphore"

	apitypes "github.com/iotexproject/iotex-core/v2/api/types"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/pkg/tracer"
	"github.com/iotexproject/iotex-core/v2/pkg/util/httputil"
)

type (
	// HTTPServer crates a http server
	HTTPServer struct {
		svr *http.Server
	}

	// hTTPHandler handles requests from http protocol
	hTTPHandler struct {
		msgHandler Web3Handler
		sem        *semaphore.Weighted
	}
)

// NewHTTPServer creates a new http server
func NewHTTPServer(route string, port int, handler http.Handler) *HTTPServer {
	if port == 0 {
		return nil
	}
	mux := http.NewServeMux()
	mux.Handle("/"+route, handler)

	svr := httputil.NewServer(":"+strconv.Itoa(port), mux, httputil.ReadHeaderTimeout(10*time.Second))
	return &HTTPServer{
		svr: &svr,
	}
}

// Start starts the http server
func (hSvr *HTTPServer) Start(_ context.Context) error {
	go func() {
		if err := hSvr.svr.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.L().Fatal("Node failed to serve.", zap.Error(err))
		}
	}()
	return nil
}

// Stop stops the http server
func (hSvr *HTTPServer) Stop(ctx context.Context) error {
	return hSvr.svr.Shutdown(ctx)
}

// newHTTPHandler creates a new http handler
func newHTTPHandler(web3Handler Web3Handler) *hTTPHandler {
	return &hTTPHandler{
		msgHandler: web3Handler,
		sem:        semaphore.NewWeighted(_maxRequestLimit),
	}
}

func (handler *hTTPHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Method != "POST" {
		w.Write([]byte("IoTeX RPC endpoint is ready."))
		return
	}

	ctx, span := tracer.NewSpan(req.Context(), "http")
	defer span.End()
	acquireCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := handler.sem.Acquire(acquireCtx, 1); err != nil {
		w.WriteHeader(http.StatusTooManyRequests)
		log.L().Error("fail to acquire semaphore", zap.Error(err))
		return
	}
	// TODO: add metrics
	defer handler.sem.Release(1)

	if err := handler.msgHandler.HandlePOSTReq(ctx, req.Body,
		apitypes.NewResponseWriter(
			func(resp interface{}) (int, error) {
				w.Header().Set("Access-Control-Allow-Origin", "*")
				w.Header().Set("Content-Type", "application/json; charset=UTF-8")
				raw, err := json.Marshal(resp)
				if err != nil {
					return 0, err
				}
				return w.Write(raw)
			}),
	); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.T(ctx).Error("fail to respond request.", zap.Error(err))
	}
}
