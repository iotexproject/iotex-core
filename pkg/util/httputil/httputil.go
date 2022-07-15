package httputil

import (
	"net"
	"net/http"
	"time"

	"golang.org/x/net/netutil"
)

const (
	_connectionCount = 400
)

type (
	// ServerOption is a server option
	ServerOption func(*serverConfig)

	serverConfig struct {
		ReadHeaderTimeout time.Duration
		ReadTimeout       time.Duration
		WriteTimeout      time.Duration
		IdleTimeout       time.Duration
	}
)

// DefaultServerConfig is the default server config
var DefaultServerConfig = serverConfig{
	ReadHeaderTimeout: 5 * time.Second,
	ReadTimeout:       30 * time.Second,
	WriteTimeout:      30 * time.Second,
	IdleTimeout:       120 * time.Second,
}

// ReadHeaderTimeout sets header timeout
func ReadHeaderTimeout(h time.Duration) ServerOption {
	return func(cfg *serverConfig) {
		cfg.ReadHeaderTimeout = h
	}
}

// NewServer creates a HTTP server with time out settings.
func NewServer(addr string, handler http.Handler, opts ...ServerOption) http.Server {
	cfg := DefaultServerConfig
	for _, opt := range opts {
		opt(&cfg)
	}

	return http.Server{
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		Addr:              addr,
		Handler:           handler,
	}
}

// LimitListener creates a tcp keep-alive listener with 400 maximum connections.
func LimitListener(addr string) (net.Listener, error) {
	if addr == "" {
		addr = ":http"
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return ln, err
	}
	return netutil.LimitListener(tcpKeepAliveListener{ln.(*net.TCPListener)}, _connectionCount), nil
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

	if err = tc.SetKeepAlive(true); err != nil {
		return nil, err
	}

	if err = tc.SetKeepAlivePeriod(3 * time.Minute); err != nil {
		return nil, err
	}

	return tc, nil
}
