// Package memconn provides an in-memory network connections. This allows
// applications to connect to themselves without having to open up ports on the
// network.
package memconn

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"
)

const connBufferSize = 64 // 1KB buffer size

// Conn is an in-memory connection. Every Conn has a remote peer representing
// the other side of the connection. Writes to Conn will be sent to the peer's
// buffer for reading.
//
// conns use a 1kB buffer where writes can be sent immediately. Writes beyond
// the buffer size will be blocked until they are read by the peer.
type Conn struct {
	peerCh chan struct{} // Closed when peer is set
	peer   *Conn

	cnd                *sync.Cond
	buf                bytes.Buffer
	readTimeout        time.Time
	readTimeoutCancel  context.CancelFunc
	writeTimeout       time.Time
	writeTimeoutCancel context.CancelFunc
	closed             bool
	name               string
}

var _ net.Conn = (*Conn)(nil)

func newConn() *Conn {
	var mut sync.Mutex
	return &Conn{
		peerCh: make(chan struct{}),
		cnd:    sync.NewCond(&mut),
		name:   "memory",
	}
}

// Attach sets the remote peer. Panics if called more than once.
func (c *Conn) Attach(peer *Conn) {
	select {
	default:
		c.peer = peer
		close(c.peerCh)
	case <-c.peerCh:
		panic("peer already set")
	}
}

// WaitPeer waits for a peer to be set or until ctx is canceled.
func (c *Conn) WaitPeer(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-c.peerCh:
		return nil
	}
}

func (c *Conn) Read(b []byte) (n int, err error) {
	for n == 0 {
		n2, err := c.readOrBlock(b)
		if err != nil {
			return n2, err
		}
		n += n2
	}
	c.cnd.Signal() // Wake up calls to .Write
	return n, nil
}

func (c *Conn) readOrBlock(b []byte) (int, error) {
	c.cnd.L.Lock()
	defer c.cnd.L.Unlock()

	if !c.readTimeout.IsZero() && !time.Now().Before(c.readTimeout) {
		return 0, os.ErrDeadlineExceeded
	}

	n, err := c.buf.Read(b)

	// We expect to get EOF from our buffer whenever there's no pending data. We
	// don't want to propagate the EOF to our reader until the conn itself is
	// closed.
	if errors.Is(err, io.EOF) {
		if c.closed {
			return n, err
		}

		// Wait until we're woken up by something, either because there's a timeout
		// or there's data to read. Spurious wakeups may happen, which would
		// eventually cause us to re-enter the wait.
		c.cnd.Wait()
	}
	return n, nil
}

func (c *Conn) Write(b []byte) (n int, err error) {
	for len(b) > 0 {
		n2, err := c.writeOrBlock(b)
		if err != nil {
			return n + n2, err
		}
		n += n2
		b = b[n2:]
	}
	return n, nil
}

func (c *Conn) writeOrBlock(b []byte) (int, error) {
	if err := c.writeAvail(); err != nil {
		return 0, err
	}
	return c.peer.enqueueOrBlock(b)
}

// writeAvail returns nil when writing is available.
func (c *Conn) writeAvail() error {
	c.cnd.L.Lock()
	defer c.cnd.L.Unlock()

	switch {
	case c.closed:
		return net.ErrClosed
	case !c.writeTimeout.IsZero() && !time.Now().Before(c.writeTimeout):
		return os.ErrDeadlineExceeded
	default:
		return nil
	}
}

// enqueueOrBlock is invoked by a peer and writes b into the local buffer.
func (c *Conn) enqueueOrBlock(b []byte) (int, error) {
	c.cnd.L.Lock()
	defer c.cnd.L.Unlock()
	if c.closed {
		return 0, net.ErrClosed
	}

	// Try to write as much as possible.
	n := len(b)
	limit := connBufferSize - c.buf.Len()
	if limit < n {
		n = limit
	}

	if n == 0 {
		// Buffer is completely full; wait for it to free up.
		c.cnd.Wait()
		return 0, nil
	}

	c.buf.Write(b[:n])
	c.cnd.Signal() // Signal that data can be read
	return n, nil
}

// Close closes both sides of the connection.
func (c *Conn) Close() error {
	err := c.handleClose()
	_ = c.peer.handleClose()
	return err
}

func (c *Conn) handleClose() error {
	c.cnd.L.Lock()
	defer c.cnd.L.Unlock()

	if c.closed {
		return nil
	}
	c.closed = true

	if c.readTimeoutCancel != nil {
		c.readTimeoutCancel()
		c.readTimeoutCancel = nil
	}
	if c.writeTimeoutCancel != nil {
		c.writeTimeoutCancel()
		c.writeTimeoutCancel = nil
	}
	c.broadcast()
	return nil
}

// broadcast will wake up all sleeping goroutines on the local and peer
// connections.
func (c *Conn) broadcast() {
	// We *MUST* wake up goroutines waiting on both the local and remote
	// condition variables because usage of a conn depends on both.
	c.cnd.Broadcast()
	c.peer.cnd.Broadcast()
}

func (c *Conn) LocalAddr() net.Addr {
	return Addr{
		name: c.name,
	}
}

func (c *Conn) RemoteAddr() net.Addr {
	return c.peer.LocalAddr()
}

func (c *Conn) SetDeadline(t time.Time) error {
	var firstError error
	if err := c.SetReadDeadline(t); err != nil {
		firstError = err
	}
	if err := c.SetWriteDeadline(t); err != nil && firstError == nil {
		firstError = err
	}
	return firstError
}

func (c *Conn) SetReadDeadline(t time.Time) error {

	c.cnd.L.Lock()
	defer c.cnd.L.Unlock()
	if c.closed {
		return fmt.Errorf("conn closed")
	}

	c.readTimeout = t

	// There should only be one deadline goroutine at a time, so cancel it if it
	// already exists.
	if c.readTimeoutCancel != nil {
		c.readTimeoutCancel()
		c.readTimeoutCancel = nil
	}
	c.readTimeoutCancel = c.deadlineTimer(t)
	return nil
}

func (c *Conn) deadlineTimer(t time.Time) context.CancelFunc {
	if t.IsZero() {
		// Deadline of zero means to wait forever.
		return nil
	}
	if t.Before(time.Now()) {
		c.broadcast()
	}
	ctx, cancel := context.WithDeadline(context.Background(), t)
	go func() {
		<-ctx.Done()
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			c.broadcast()
		}
	}()
	return cancel
}

func (c *Conn) SetWriteDeadline(t time.Time) error {
	c.cnd.L.Lock()
	defer c.cnd.L.Unlock()
	if c.closed {
		return fmt.Errorf("conn closed")
	}

	c.writeTimeout = t

	// There should only be one deadline goroutine at a time, so cancel it if it
	// already exists.
	if c.writeTimeoutCancel != nil {
		c.writeTimeoutCancel()
		c.writeTimeoutCancel = nil
	}
	c.writeTimeoutCancel = c.deadlineTimer(t)
	return nil
}
