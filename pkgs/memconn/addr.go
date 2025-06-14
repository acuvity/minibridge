package memconn

import "net"

// Addr is the address of a memlistener.
type Addr struct {
	name string
}

var _ net.Addr = (*Addr)(nil)

// Network implements net.Addr. Returns "memory."
func (Addr) Network() string { return "memory" }

// String implements net.Addr. Returns "memory."
func (a Addr) String() string { return a.name }
