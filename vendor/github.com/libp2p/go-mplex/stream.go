package multiplex

import (
	"context"
	"errors"
	"io"
	"sync"
	"time"

	pool "github.com/libp2p/go-buffer-pool"
	streammux "github.com/libp2p/go-stream-muxer"
)

// streamID is a convenience type for operating on stream IDs
type streamID struct {
	id        uint64
	initiator bool
}

// header computes the header for the given tag
func (id *streamID) header(tag uint64) uint64 {
	header := id.id<<3 | tag
	if !id.initiator {
		header--
	}
	return header
}

type Stream struct {
	id     streamID
	name   string
	dataIn chan []byte
	mp     *Multiplex

	extra []byte

	// exbuf is for holding the reference to the beginning of the extra slice
	// for later memory pool freeing
	exbuf []byte

	wDeadline time.Time
	rDeadline time.Time

	clLock       sync.Mutex
	closedRemote bool

	// Closed when the connection is reset.
	reset chan struct{}

	// Closed when the writer is closed (reset will also be closed)
	closedLocal  context.Context
	doCloseLocal context.CancelFunc
}

func (s *Stream) Name() string {
	return s.name
}

func (s *Stream) waitForData(ctx context.Context) error {
	if !s.rDeadline.IsZero() {
		dctx, cancel := context.WithDeadline(ctx, s.rDeadline)
		defer cancel()
		ctx = dctx
	}

	select {
	case <-s.reset:
		// This is the only place where it's safe to return these.
		s.returnBuffers()
		return streammux.ErrReset
	case read, ok := <-s.dataIn:
		if !ok {
			return io.EOF
		}
		s.extra = read
		s.exbuf = read
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Stream) returnBuffers() {
	if s.exbuf != nil {
		pool.Put(s.exbuf)
		s.exbuf = nil
		s.extra = nil
	}
	for {
		select {
		case read, ok := <-s.dataIn:
			if !ok {
				return
			}
			if read == nil {
				continue
			}
			pool.Put(read)
		default:
			return
		}
	}
}

func (s *Stream) Read(b []byte) (int, error) {
	if s.extra == nil {
		err := s.waitForData(context.Background())
		if err != nil {
			return 0, err
		}
	}
	n := copy(b, s.extra)
	if n < len(s.extra) {
		s.extra = s.extra[n:]
	} else {
		if s.exbuf != nil {
			pool.Put(s.exbuf)
		}
		s.extra = nil
		s.exbuf = nil
	}
	return n, nil
}

func (s *Stream) Write(b []byte) (int, error) {
	var written int
	for written < len(b) {
		wl := len(b) - written
		if wl > MaxMessageSize {
			wl = MaxMessageSize
		}

		n, err := s.write(b[written : written+wl])
		if err != nil {
			return written, err
		}

		written += n
	}

	return written, nil
}

func (s *Stream) write(b []byte) (int, error) {
	if s.isClosed() {
		return 0, errors.New("cannot write to closed stream")
	}

	wDeadlineCtx, cleanup := func(s *Stream) (context.Context, context.CancelFunc) {
		if s.wDeadline.IsZero() {
			return s.closedLocal, nil
		} else {
			return context.WithDeadline(s.closedLocal, s.wDeadline)
		}
	}(s)

	err := s.mp.sendMsg(wDeadlineCtx, s.id.header(messageTag), b)

	if cleanup != nil {
		cleanup()
	}

	if err != nil {
		if err == context.Canceled {
			err = errors.New("cannot write to closed stream")
		}
		return 0, err
	}

	return len(b), nil
}

func (s *Stream) isClosed() bool {
	return s.closedLocal.Err() != nil
}

func (s *Stream) Close() error {
	err := s.mp.sendMsg(context.Background(), s.id.header(closeTag), nil)

	if s.isClosed() {
		return nil
	}

	s.clLock.Lock()
	remote := s.closedRemote
	s.clLock.Unlock()

	s.doCloseLocal()

	if remote {
		s.mp.chLock.Lock()
		delete(s.mp.channels, s.id)
		s.mp.chLock.Unlock()
	}

	return err
}

func (s *Stream) Reset() error {
	s.clLock.Lock()
	isClosed := s.isClosed()
	if s.closedRemote && isClosed {
		s.clLock.Unlock()
		return nil
	}

	if !s.closedRemote {
		close(s.reset)
		// We generally call this to tell the other side to go away. No point in waiting around.
		go s.mp.sendMsg(context.Background(), s.id.header(resetTag), nil)
	}

	s.doCloseLocal()

	s.closedRemote = true

	s.clLock.Unlock()

	s.mp.chLock.Lock()
	delete(s.mp.channels, s.id)
	s.mp.chLock.Unlock()

	return nil
}

func (s *Stream) SetDeadline(t time.Time) error {
	s.rDeadline = t
	s.wDeadline = t
	return nil
}

func (s *Stream) SetReadDeadline(t time.Time) error {
	s.rDeadline = t
	return nil
}

func (s *Stream) SetWriteDeadline(t time.Time) error {
	s.wDeadline = t
	return nil
}
