package client

import (
	"fmt"
	"math"
)

type creds struct {
	Uid    uint32
	Gid    uint32
	Groups []uint32
}

type stdioCfg struct {
	useTempDir bool
	creds      *creds
}

func newStdioCfg() stdioCfg {
	return stdioCfg{}
}

// An StdioOption can be passed to the Client.
type StdioOption func(*stdioCfg)

// OptStdioUseTempDir defines if the the client should
// run the command into it's own working dir. If false,
// the command will run in minibridge current cwd
func OptStdioUseTempDir(use bool) StdioOption {
	return func(c *stdioCfg) {
		c.useTempDir = use
	}
}

// OptStdioCredentials sets the uid and gid to run the command as.
func OptStdioCredentials(uid int, gid int, groups []int) StdioOption {
	return func(c *stdioCfg) {

		grps := make([]uint32, 0, len(groups))

		for i, g := range groups {

			if g < 0 {
				continue
			}

			if g > math.MaxUint32 {
				panic(fmt.Sprintf("invalid group %d. overflows", i))
			}

			grps = append(grps, uint32(g)) // #nosec: G115
		}

		if len(grps) == 0 && uid < 0 && gid < 0 {
			return
		}

		c.creds = &creds{Groups: grps}

		if uid > -1 {

			if uid > math.MaxUint32 {
				panic("invalid uid. overflows")
			}

			c.creds.Uid = uint32(uid) // #nosec: G115
		}

		if gid > -1 {

			if gid > math.MaxUint32 {
				panic("invalid gid. overflows")
			}

			c.creds.Gid = uint32(gid) // #nosec: G115
		}
	}
}
