package api

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	apitypes "github.com/iotexproject/iotex-core/api/types"
	"github.com/iotexproject/iotex-core/pkg/log"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 15 * 1024 * 1024
)

// WebsocketServer handles requests from websocket protocol
type WebsocketServer struct {
	svr        *http.Server
	msgHandler Web3Handler
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// NewWebSocketServer creates a new websocket server
func NewWebSocketServer(route string, port int, handler Web3Handler) *WebsocketServer {
	if port == 0 {
		return nil
	}
	svr := &WebsocketServer{
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

// Start starts the websocket server
func (wsSvr *WebsocketServer) Start(_ context.Context) error {
	go func() {
		if err := wsSvr.svr.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.L().Fatal("Node failed to serve.", zap.Error(err))
		}
	}()
	return nil
}

// Stop stops the websocket server
func (wsSvr *WebsocketServer) Stop(_ context.Context) error {
	return wsSvr.svr.Shutdown(context.Background())
}

func (wsSvr *WebsocketServer) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	upgrader.CheckOrigin = func(_ *http.Request) bool { return true }

	// upgrade this connection to a WebSocket connection
	ws, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		log.Logger("api").Warn("failed to upgrade http server to websocket", zap.Error(err))
		return
	}

	wsSvr.handleConnection(ws)
}

func (wsSvr *WebsocketServer) handleConnection(ws *websocket.Conn) {
	defer ws.Close()
	ws.SetReadDeadline(time.Now().Add(pongWait))
	ws.SetReadLimit(maxMessageSize)
	ws.SetPongHandler(func(string) error {
		ws.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	go ping(ctx, ws, cancel)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			_, reader, err := ws.NextReader()
			if err != nil {
				log.Logger("api").Debug("Client Disconnected", zap.Error(err))
				cancel()
				return
			}

			err = wsSvr.msgHandler.HandlePOSTReq(reader,
				apitypes.NewResponseWriter(
					func(resp interface{}) error {
						ws.SetWriteDeadline(time.Now().Add(writeWait))
						return ws.WriteJSON(resp)
					}),
			)
			if err != nil {
				log.Logger("api").Warn("fail to respond request.", zap.Error(err))
				cancel()
				return
			}
		}
	}
}

func ping(ctx context.Context, ws *websocket.Conn, cancel context.CancelFunc) {
	pingTicker := time.NewTicker(pingPeriod)
	defer func() {
		pingTicker.Stop()
		ws.Close()
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-pingTicker.C:
			ws.SetWriteDeadline(time.Now().Add(writeWait))
			if err := ws.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				log.Logger("api").Warn("fail to respond request.", zap.Error(err))
				cancel()
				return
			}
		}
	}
}
