package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/pkg/log"
)

// HTTPServer handles requests from http protocol
type HTTPServer struct {
	svr        *http.Server
	msgHandler Web3Handler
}

// NewHTTPServer creates a new http server
func NewHTTPServer(route string, port int, handler Web3Handler) *HTTPServer {
	if port == 0 {
		return nil
	}
	svr := &HTTPServer{
		svr: &http.Server{
			Addr: ":" + strconv.Itoa(port),
		},
		msgHandler: handler,
	}

	mux := http.NewServeMux()
	mux.Handle("/"+route, svr)
	svr.svr.Handler = mux
	return svr
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
func (hSvr *HTTPServer) Stop(_ context.Context) error {
	return hSvr.svr.Shutdown(context.Background())
}

func (hSvr *HTTPServer) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	httpResp := hSvr.msgHandler.HandlePOSTReq(req.Body)

	// write results into http reponse
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	if err := json.NewEncoder(w).Encode(httpResp); err != nil {
		log.Logger("api").Warn("fail to respond request.", zap.Error(err))
	}
}
