package netutil

import (
	"net"
	"time"

	"golang.org/x/net/netutil"
)

const connectionCount = 400

// LimitHTTPListener creates a tcp keep-alive listener with 400 maximum connections.
func LimitHTTPListener(addr string) (net.Listener, error) {
	if addr == "" {
		addr = ":http"
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return ln, err
	}
	return netutil.LimitListener(tcpKeepAliveListener{ln.(*net.TCPListener)}, connectionCount), nil
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
