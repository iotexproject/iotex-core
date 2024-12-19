package api

import (
	"context"
	"math"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
	"golang.org/x/time/rate"

	apitypes "github.com/iotexproject/iotex-core/v2/api/types"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/pkg/tracer"
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
	coreService CoreService
	msgHandler  Web3Handler
	limiter     *rate.Limiter
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// type safeWebsocketConn wraps websocket.Conn with a mutex
// to avoid concurrent write to the connection
// https://pkg.go.dev/github.com/gorilla/websocket#hdr-Concurrency
type safeWebsocketConn struct {
	ws *websocket.Conn
	mu sync.Mutex
}

// WriteJSON writes a JSON message to the connection in a thread-safe way
func (c *safeWebsocketConn) WriteJSON(message interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ws.WriteJSON(message)
}

// WriteMessage writes a message to the connection in a thread-safe way
func (c *safeWebsocketConn) WriteMessage(messageType int, data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ws.WriteMessage(messageType, data)
}

// Close closes the underlying network connection without sending or waiting for a close frame
func (c *safeWebsocketConn) Close() error {
	return c.ws.Close()
}

// SetWriteDeadline sets the write deadline on the underlying network connection
func (c *safeWebsocketConn) SetWriteDeadline(t time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ws.SetWriteDeadline(t)
}

// NewWebsocketHandler creates a new websocket handler
func NewWebsocketHandler(coreService CoreService, web3Handler Web3Handler, limiter *rate.Limiter) *WebsocketHandler {
	if limiter == nil {
		// set the limiter to the maximum possible rate
		limiter = rate.NewLimiter(rate.Limit(math.MaxFloat64), 1)
	}
	return &WebsocketHandler{
		msgHandler:  web3Handler,
		limiter:     limiter,
		coreService: coreService,
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

	wsSvr.handleConnection(req.Context(), ws)
}

func (wsSvr *WebsocketHandler) handleConnection(ctx context.Context, ws *websocket.Conn) {
	defer ws.Close()
	if err := ws.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		log.Logger("api").Warn("failed to set read deadline timeout.", zap.Error(err))
	}
	ws.SetReadLimit(maxMessageSize)
	ws.SetPongHandler(func(string) error {
		if err := ws.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
			log.Logger("api").Warn("failed to set read deadline timeout.", zap.Error(err))
		}
		return nil
	})

	ctx, cancel := context.WithCancel(WithStreamContext(ctx))
	safeWs := &safeWebsocketConn{ws: ws}
	go ping(ctx, safeWs, cancel)

	defer func() {
		// clean up the stream context
		sc, _ := StreamFromContext(ctx)
		for _, id := range sc.ListenerIDs() {
			wsSvr.coreService.ChainListener().RemoveResponder(id)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			if err := wsSvr.limiter.Wait(ctx); err != nil {
				cancel()
				return
			}
			_, reader, err := ws.NextReader()
			if err != nil {
				log.Logger("api").Debug("Client Disconnected", zap.Error(err))
				cancel()
				return
			}
			func() {
				wsCtx, span := tracer.NewSpan(ctx, "wss")
				defer span.End()
				err = wsSvr.msgHandler.HandlePOSTReq(wsCtx, reader,
					apitypes.NewResponseWriter(
						func(resp interface{}) (int, error) {
							if err = safeWs.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
								log.T(wsCtx).Warn("failed to set write deadline timeout.", zap.Error(err))
							}
							return 0, safeWs.WriteJSON(resp)
						}),
				)
				if err != nil {
					log.T(wsCtx).Warn("fail to respond request.", zap.Error(err))
					cancel()
					return
				}
			}()
		}
	}
}

func ping(ctx context.Context, ws *safeWebsocketConn, cancel context.CancelFunc) {
	pingTicker := time.NewTicker(pingPeriod)
	defer func() {
		pingTicker.Stop()
		if err := ws.Close(); err != nil {
			log.Logger("api").Warn("fail to close websocket connection.", zap.Error(err))
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-pingTicker.C:
			if err := ws.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				log.Logger("api").Warn("failed to set write deadline timeout.", zap.Error(err))
			}
			if err := ws.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				log.Logger("api").Warn("fail to respond request.", zap.Error(err))
				cancel()
				return
			}
		}
	}
}
