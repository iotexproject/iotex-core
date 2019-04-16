package ssdp

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
)

// Monitor monitors SSDP's alive and byebye messages.
type Monitor struct {
	alive AliveHandler
	bye   ByeHandler
	conn  net.PacketConn
	wg    sync.WaitGroup
}

// NewMonitor creates a new Monitor.
func NewMonitor(alive AliveHandler, bye ByeHandler) (*Monitor, error) {
	if alive == nil {
		alive = nullAlive
	}
	if bye == nil {
		bye = nullBye
	}
	conn, err := multicastListen("0.0.0.0:1900")
	if err != nil {
		return nil, err
	}
	logf("monitoring on %s", conn.LocalAddr().String())
	m := &Monitor{
		alive: alive,
		bye:   bye,
		conn:  conn,
	}
	m.wg.Add(1)
	go func() {
		m.serve()
		m.wg.Done()
	}()
	return m, nil
}

func (m *Monitor) serve() error {
	buf := make([]byte, 65535)
	for {
		n, addr, err := m.conn.ReadFrom(buf)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		msg := make([]byte, n)
		copy(msg, buf[:n])
		go m.handleRaw(addr, msg)
	}
}

func (m *Monitor) handleRaw(addr net.Addr, raw []byte) error {
	req, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(raw)))
	if err != nil {
		return err
	}
	switch nts := req.Header.Get("NTS"); nts {
	case "ssdp:alive":
		if req.Method != "NOTIFY" {
			return fmt.Errorf("unexpected method for %q: %s", "ssdp:alive", req.Method)
		}
		m.alive(&Alive{
			From:      addr,
			Type:      req.Header.Get("NT"),
			USN:       req.Header.Get("USN"),
			Location:  req.Header.Get("LOCATION"),
			Server:    req.Header.Get("SERVER"),
			rawHeader: req.Header,
		})
	case "ssdp:byebye":
		if req.Method != "NOTIFY" {
			return fmt.Errorf("unexpected method for %q: %s", "ssdp:byebye", req.Method)
		}
		m.bye(&Bye{
			From:      addr,
			Type:      req.Header.Get("NT"),
			USN:       req.Header.Get("USN"),
			rawHeader: req.Header,
		})
	default:
		return fmt.Errorf("unknown NTS: %s", nts)
	}
	return nil
}

// Close closes monitoring.
func (m *Monitor) Close() error {
	if m.conn != nil {
		m.conn.Close()
		m.conn = nil
		m.wg.Wait()
	}
	return nil
}

// Alive represents SSDP's ssdp:alive message.
type Alive struct {
	// From is a sender of this message
	From net.Addr

	// Type is a property of "NT"
	Type string

	// USN is a property of "USN"
	USN string

	// Location is a property of "LOCATION"
	Location string

	// Server is a property of "SERVER"
	Server string

	rawHeader http.Header
	maxAge    *int
}

// Header returns all properties in response of search.
func (m *Alive) Header() http.Header {
	return m.rawHeader
}

// MaxAge extracts "max-age" value from "CACHE-CONTROL" property.
func (m *Alive) MaxAge() int {
	if m.maxAge == nil {
		m.maxAge = new(int)
		*m.maxAge = extractMaxAge(m.rawHeader.Get("CACHE-CONTROL"), -1)
	}
	return *m.maxAge
}

// AliveHandler is handler of Alive message.
type AliveHandler func(*Alive)

func nullAlive(*Alive) {
	// nothing to do.
}

// Bye represents SSDP's ssdp:byebye message.
type Bye struct {
	// From is a sender of this message
	From net.Addr

	// Type is a property of "NT"
	Type string

	// USN is a property of "USN"
	USN string

	rawHeader http.Header
}

// Header returns all properties in response of search.
func (m *Bye) Header() http.Header {
	return m.rawHeader
}

// ByeHandler is handler of Bye message.
type ByeHandler func(*Bye)

func nullBye(*Bye) {
	// nothing to do.
}
