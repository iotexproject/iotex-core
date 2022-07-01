package api

import (
	"context"
	"net/http"
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

// WebsocketHandler handles requests from websocket protocol
type WebsocketHandler struct {
	msgHandler Web3Handler
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// NewWebsocketHandler creates a new websocket handler
func NewWebsocketHandler(web3Handler Web3Handler) *WebsocketHandler {
	return &WebsocketHandler{
		msgHandler: web3Handler,
	}
}

func (wsSvr *WebsocketHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	upgrader.CheckOrigin = func(_ *http.Request) bool { return true }

	// upgrade this connection to a WebSocket connection
	ws, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		log.Logger("api").Warn("failed to upgrade http server to websocket", zap.Error(err))
		return
	}

	wsSvr.handleConnection(ws)
}

func (wsSvr *WebsocketHandler) handleConnection(ws *websocket.Conn) {
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
