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

// Server creates a HTTP server with time out settings.
func Server(addr string, handler http.Handler) http.Server {
	return http.Server{
		ReadTimeout:  _readTimeout,
		WriteTimeout: _writeTimeout,
		IdleTimeout:  _idleTimeout,
		Addr:         addr,
		Handler:      handler,
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
	tc.SetKeepAlive(true)
	tc.SetKeepAlivePeriod(3 * time.Minute)
	return tc, nil
}
