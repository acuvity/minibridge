package memconn

import (
	"context"
	"fmt"
	"net"
)

// Listener is an in-memory net.Listener. Call DialContext to create a new
// connection.
type Listener struct {
	pending chan *Conn
	closed  chan struct{}
}

var _ net.Listener = (*Listener)(nil)

// NewListener creates a new in-memory Listener.
func NewListener() *Listener {
	return &Listener{
		pending: make(chan *Conn),
		closed:  make(chan struct{}),
	}
}

// Accept waits for and returns the next connection to l. Connections to l are
// established by calling l.DialContext.
//
// The returned net.Conn is the server side of the connection.
func (l *Listener) Accept() (net.Conn, error) {
	select {
	case peer := <-l.pending:
		local := newConn()
		peer.Attach(local)
		local.Attach(peer)
		return local, nil

	case <-l.closed:
		return nil, fmt.Errorf("Listener closed")
	}
}

// Close closes l. Any blocked Accept operations will immediately be unblocked
// and return errors. Already Accepted connections are not closed.
func (l *Listener) Close() error {
	select {
	default:
		close(l.closed)
		return nil
	case <-l.closed:
		return fmt.Errorf("already closed")
	}
}

// Addr returns l's address. This will always be a fake "memory"
// address.
func (l *Listener) Addr() net.Addr {
	return Addr{}
}

// DialContext creates a new connection to l. DialContext will block until the
// connection is accepted through a blocked l.Accept call or until ctx is
// canceled.
//
// Note that unlike other Dial methods in different packages, there is no
// address to supply because the remote side of the connection is always the
// in-memory listener.
func (l *Listener) DialContext(ctx context.Context, clientName string) (net.Conn, error) {
	local := newConn()
	local.name = clientName

	select {
	case l.pending <- local:
		// Wait for our peer to be connected.
		if err := local.WaitPeer(ctx); err != nil {
			return nil, err
		}
		return local, nil
	case <-l.closed:
		return nil, fmt.Errorf("server closed")
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
