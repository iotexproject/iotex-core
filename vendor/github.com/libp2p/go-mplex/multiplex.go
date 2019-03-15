package multiplex

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	logging "github.com/ipfs/go-log"
	pool "github.com/libp2p/go-buffer-pool"
)

var log = logging.Logger("mplex")

var MaxMessageSize = 1 << 20

// Max time to block waiting for a slow reader to read from a stream before
// resetting it. Preferably, we'd have some form of back-pressure mechanism but
// we don't have that in this protocol.
var ReceiveTimeout = 5 * time.Second

// ErrShutdown is returned when operating on a shutdown session
var ErrShutdown = errors.New("session shut down")

// ErrTwoInitiators is returned when both sides think they're the initiator
var ErrTwoInitiators = errors.New("two initiators")

// ErrInvalidState is returned when the other side does something it shouldn't.
// In this case, we close the connection to be safe.
var ErrInvalidState = errors.New("received an unexpected message from the peer")

// +1 for initiator
const (
	newStreamTag = 0
	messageTag   = 2
	closeTag     = 4
	resetTag     = 6
)

// Multiplex is a mplex session.
type Multiplex struct {
	con       net.Conn
	buf       *bufio.Reader
	nextID    uint64
	initiator bool

	closed       chan struct{}
	shutdown     chan struct{}
	shutdownErr  error
	shutdownLock sync.Mutex

	wrTkn chan struct{}

	nstreams chan *Stream

	channels map[streamID]*Stream
	chLock   sync.Mutex
}

// NewMultiplex creates a new multiplexer session.
func NewMultiplex(con net.Conn, initiator bool) *Multiplex {
	mp := &Multiplex{
		con:       con,
		initiator: initiator,
		buf:       bufio.NewReader(con),
		channels:  make(map[streamID]*Stream),
		closed:    make(chan struct{}),
		shutdown:  make(chan struct{}),
		wrTkn:     make(chan struct{}, 1),
		nstreams:  make(chan *Stream, 16),
	}

	go mp.handleIncoming()

	mp.wrTkn <- struct{}{}

	return mp
}

func (mp *Multiplex) newStream(id streamID, name string) (s *Stream) {
	s = &Stream{
		id:     id,
		name:   name,
		dataIn: make(chan []byte, 8),
		reset:  make(chan struct{}),
		mp:     mp,
	}

	s.closedLocal, s.doCloseLocal = context.WithCancel(context.Background())
	return
}

// Accept accepts the next stream from the connection.
func (m *Multiplex) Accept() (*Stream, error) {
	select {
	case s, ok := <-m.nstreams:
		if !ok {
			return nil, errors.New("multiplex closed")
		}
		return s, nil
	case <-m.closed:
		return nil, m.shutdownErr
	}
}

// Close closes the session.
func (mp *Multiplex) Close() error {
	mp.closeNoWait()

	// Wait for the receive loop to finish.
	<-mp.closed

	return nil
}

func (mp *Multiplex) closeNoWait() {
	mp.shutdownLock.Lock()
	select {
	case <-mp.shutdown:
	default:
		mp.con.Close()
		close(mp.shutdown)
	}
	mp.shutdownLock.Unlock()
}

// IsClosed returns true if the session is closed.
func (mp *Multiplex) IsClosed() bool {
	select {
	case <-mp.closed:
		return true
	default:
		return false
	}
}

func (mp *Multiplex) sendMsg(ctx context.Context, header uint64, data []byte) error {
	select {
	case tkn := <-mp.wrTkn:
		defer func() { mp.wrTkn <- tkn }()
	case <-ctx.Done():
		return ctx.Err()
	}

	dl, hasDl := ctx.Deadline()
	if hasDl {
		if err := mp.con.SetWriteDeadline(dl); err != nil {
			return err
		}
	}
	buf := pool.Get(len(data) + 20)
	defer pool.Put(buf)

	n := 0
	n += binary.PutUvarint(buf[n:], header)
	n += binary.PutUvarint(buf[n:], uint64(len(data)))
	n += copy(buf[n:], data)

	written, err := mp.con.Write(buf[:n])
	if err != nil && written > 0 {
		// Bail. We've written partial message and can't do anything
		// about this.
		mp.con.Close()
		return err
	}

	if hasDl {
		// only return this error if we don't *already* have an error from the write.
		if err2 := mp.con.SetWriteDeadline(time.Time{}); err == nil && err2 != nil {
			return err2
		}
	}

	return err
}

func (mp *Multiplex) nextChanID() uint64 {
	out := mp.nextID
	mp.nextID++
	return out
}

// NewStream creates a new stream.
func (mp *Multiplex) NewStream() (*Stream, error) {
	return mp.NewNamedStream("")
}

// NewNamedStream creates a new named stream.
func (mp *Multiplex) NewNamedStream(name string) (*Stream, error) {
	mp.chLock.Lock()

	// We could call IsClosed but this is faster (given that we already have
	// the lock).
	if mp.channels == nil {
		mp.chLock.Unlock()
		return nil, ErrShutdown
	}

	sid := mp.nextChanID()
	header := (sid << 3) | newStreamTag

	if name == "" {
		name = fmt.Sprint(sid)
	}
	s := mp.newStream(streamID{
		id:        sid,
		initiator: true,
	}, name)
	mp.channels[s.id] = s
	mp.chLock.Unlock()

	err := mp.sendMsg(context.Background(), header, []byte(name))
	if err != nil {
		return nil, err
	}

	return s, nil
}

func (mp *Multiplex) cleanup() {
	mp.closeNoWait()
	mp.chLock.Lock()
	defer mp.chLock.Unlock()
	for _, msch := range mp.channels {
		msch.clLock.Lock()
		if !msch.closedRemote {
			msch.closedRemote = true
			// Cancel readers
			close(msch.reset)
		}

		msch.doCloseLocal()
		msch.clLock.Unlock()
	}
	// Don't remove this nil assignment. We check if this is nil to check if
	// the connection is closed when we already have the lock (faster than
	// checking if the stream is closed).
	mp.channels = nil
	if mp.shutdownErr == nil {
		mp.shutdownErr = ErrShutdown
	}
	close(mp.closed)
}

func (mp *Multiplex) handleIncoming() {
	defer mp.cleanup()

	recvTimeout := time.NewTimer(0)
	defer recvTimeout.Stop()

	if !recvTimeout.Stop() {
		<-recvTimeout.C
	}

	for {
		chID, tag, err := mp.readNextHeader()
		if err != nil {
			mp.shutdownErr = err
			return
		}

		remoteIsInitiator := tag&1 == 0
		ch := streamID{
			// true if *I'm* the initiator.
			initiator: !remoteIsInitiator,
			id:        chID,
		}
		// Rounds up the tag:
		// 0 -> 0
		// 1 -> 2
		// 2 -> 2
		// 3 -> 4
		// etc...
		tag += (tag & 1)

		b, err := mp.readNext()
		if err != nil {
			mp.shutdownErr = err
			return
		}

		mp.chLock.Lock()
		msch, ok := mp.channels[ch]
		mp.chLock.Unlock()

		switch tag {
		case newStreamTag:
			if ok {
				log.Debugf("received NewStream message for existing stream: %d", ch)
				mp.shutdownErr = ErrInvalidState
				return
			}

			name := string(b)
			pool.Put(b)

			msch = mp.newStream(ch, name)
			mp.chLock.Lock()
			mp.channels[ch] = msch
			mp.chLock.Unlock()
			select {
			case mp.nstreams <- msch:
			case <-mp.shutdown:
				return
			}

		case resetTag:
			if !ok {
				// This is *ok*. We forget the stream on reset.
				continue
			}
			msch.clLock.Lock()

			// Honestly, this check should never be true... It means we've leaked.
			// However, this is an error on *our* side so we shouldn't just bail.
			isClosed := msch.isClosed()
			if isClosed && msch.closedRemote {
				msch.clLock.Unlock()
				log.Errorf("leaked a completely closed stream")
				continue
			}

			if !msch.closedRemote {
				close(msch.reset)
			}
			msch.closedRemote = true
			msch.doCloseLocal()

			msch.clLock.Unlock()

			mp.chLock.Lock()
			delete(mp.channels, ch)
			mp.chLock.Unlock()
		case closeTag:
			if !ok {
				continue
			}

			msch.clLock.Lock()

			if msch.closedRemote {
				msch.clLock.Unlock()
				// Technically a bug on the other side. We
				// should consider killing the connection.
				continue
			}

			close(msch.dataIn)
			msch.closedRemote = true

			cleanup := msch.isClosed()

			msch.clLock.Unlock()

			if cleanup {
				mp.chLock.Lock()
				delete(mp.channels, ch)
				mp.chLock.Unlock()
			}
		case messageTag:
			if !ok {
				// reset stream, return b
				pool.Put(b)

				// This is a perfectly valid case when we reset
				// and forget about the stream.
				log.Debugf("message for non-existant stream, dropping data: %d", ch)
				go mp.sendMsg(context.Background(), ch.header(resetTag), nil)
				continue
			}

			msch.clLock.Lock()
			remoteClosed := msch.closedRemote
			msch.clLock.Unlock()
			if remoteClosed {
				// closed stream, return b
				pool.Put(b)

				log.Errorf("Received data from remote after stream was closed by them. (len = %d)", len(b))
				go mp.sendMsg(context.Background(), msch.id.header(resetTag), nil)
				continue
			}

			recvTimeout.Reset(ReceiveTimeout)
			select {
			case msch.dataIn <- b:
			case <-msch.reset:
				pool.Put(b)
			case <-recvTimeout.C:
				pool.Put(b)
				log.Warningf("timed out receiving message into stream queue.")
				// Do not do this asynchronously. Otherwise, we
				// could drop a message, then receive a message,
				// then reset.
				msch.Reset()
				continue
			case <-mp.shutdown:
				pool.Put(b)
				return
			}
			if !recvTimeout.Stop() {
				<-recvTimeout.C
			}
		default:
			log.Debugf("message with unknown header on stream %s", ch)
			if ok {
				msch.Reset()
			}
		}
	}
}

func (mp *Multiplex) readNextHeader() (uint64, uint64, error) {
	h, err := binary.ReadUvarint(mp.buf)
	if err != nil {
		return 0, 0, err
	}

	// get channel ID
	ch := h >> 3

	rem := h & 7

	return ch, rem, nil
}

func (mp *Multiplex) readNext() ([]byte, error) {
	// get length
	l, err := binary.ReadUvarint(mp.buf)
	if err != nil {
		return nil, err
	}

	if l > uint64(MaxMessageSize) {
		return nil, fmt.Errorf("message size too large!")
	}

	if l == 0 {
		return nil, nil
	}

	buf := pool.Get(int(l))
	n, err := io.ReadFull(mp.buf, buf)
	if err != nil {
		return nil, err
	}

	return buf[:n], nil
}
