package httputil

import (
	"net"
	"net/http"
	"time"

	"golang.org/x/net/netutil"
)

const (
	_connectionCount = 400
	_readTimeout     = 5 * time.Second
	_writeTimeout    = 5 * time.Second
	_idleTimeout     = 120 * time.Second
)

type ServerOption func(*serverConfig)

type serverConfig struct {
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

type ListenerOption func(*listenerConfig)

func SetTimeout(r, w, i time.Duration) ServerOption {
	return func(cfg *serverConfig) {
		cfg.ReadTimeout = r
		cfg.WriteTimeout = w
		cfg.IdleTimeout = i
	}
}

type listenerConfig struct {
	ConnectionCount int
}

func SetConnectionCount(c int) ListenerOption {
	return func(cfg *listenerConfig) { cfg.ConnectionCount = c }
}

// Server creates a HTTP server with time out settings.
func Server(addr string, handler http.Handler, opts ...ServerOption) http.Server {
	cfg := serverConfig{
		ReadTimeout:  _readTimeout,
		WriteTimeout: _writeTimeout,
		IdleTimeout:  _idleTimeout,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	return http.Server{
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
		Addr:         addr,
		Handler:      handler,
	}
}

// LimitListener creates a tcp keep-alive listener with 400 maximum connections.
func LimitListener(addr string, opts ...ListenerOption) (net.Listener, error) {
	if addr == "" {
		addr = ":http"
	}
	cfg := listenerConfig{_connectionCount}
	for _, opt := range opts {
		opt(&cfg)
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return ln, err
	}
	return netutil.LimitListener(tcpKeepAliveListener{ln.(*net.TCPListener)}, cfg.ConnectionCount), nil
}

// tcpKeepAliveListener sets TCP keep-alive timeouts on accepted
// connections. It's used by ListenAndServe and ListenAndServeTLS so
// dead TCP connections (e.g. closing laptop mid-download) eventually
// go away.
// refer: https://golang.org/src/net/http/server.go
type tcpKeepAliveListener struct {
	*net.TCPListener
}

func (ln tcpKeepAliveListener) Accept() (net.Conn, error) {
	tc, err := ln.AcceptTCP()
	if err != nil {
		return nil, err
	}
	tc.SetKeepAlive(true)
	tc.SetKeepAlivePeriod(3 * time.Minute)
	return tc, nil
}
