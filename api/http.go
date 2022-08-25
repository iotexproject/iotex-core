package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"go.uber.org/zap"

	apitypes "github.com/iotexproject/iotex-core/api/types"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/util/httputil"
)

type (
	// HTTPServer crates a http server
	HTTPServer struct {
		svr *http.Server
	}

	// hTTPHandler handles requests from http protocol
	hTTPHandler struct {
		msgHandler Web3Handler
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
	}
}

func (handler *hTTPHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if err := handler.msgHandler.HandlePOSTReq(req.Body,
		apitypes.NewResponseWriter(
			func(resp interface{}) error {
				w.Header().Set("Access-Control-Allow-Origin", "*")
				w.Header().Set("Content-Type", "application/json; charset=UTF-8")
				return json.NewEncoder(w).Encode(resp)
			}),
	); err != nil {
		log.Logger("api").Warn("fail to respond request.", zap.Error(err))
	}
}
