package client

import "math"

type creds struct {
	Uid    uint32
	Gid    uint32
	Groups []uint32
}

type cfg struct {
	useTempDir bool
	creds      *creds
}

func newCfg() cfg {
	return cfg{}
}

// An Option can be passed to the Client.
type Option func(*cfg)

// OptUseTempDir defines if the the client should
// run the command into it's own working dir. If false,
// the command will run in minibridge current cwd
func OptUseTempDir(use bool) Option {
	return func(c *cfg) {
		c.useTempDir = use
	}
}

// OptCredentials sets the uid and gid to run the command as.
func OptCredentials(uid int, gid int, groups []int) Option {
	return func(c *cfg) {

		grps := make([]uint32, len(groups))

		for i, g := range groups {

			if g > math.MaxUint32 {
				panic("invalid group. overflows")
			}

			grps[i] = uint32(g) // #nosec: G115
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
