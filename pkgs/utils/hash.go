package utils

import "fmt"

// SBOM contains a list of hashes for hashable
// resources.
type SBOM struct {
	Tools Hashes `json:"tools,omitzero"`
}

// Matches return nil if both received and o
// match, meaning len are identical, and all hashes
// on h match hashes on o.
func (h SBOM) Matches(o SBOM) error {

	if err := cmpH(h.Tools, o.Tools); err != nil {
		return fmt.Errorf("invalid tool: %w", err)
	}

	return nil
}

// Hashes are a list of Hash.
type Hashes []Hash

// Map converts the Hashes into a map[string]Hash keyed by the Hash Name.
func (l Hashes) Map() map[string]Hash {

	out := make(map[string]Hash, len(l))

	for _, h := range l {
		out[h.Name] = h
	}

	return out
}

// A Hash represent the hash of an item with it's name
// and potential parameters.
type Hash struct {
	Name   string `json:"name"`
	Hash   string `json:"hash"`
	Params Hashes `json:"params,omitzero"`
}

func cmpH(a Hashes, b Hashes) error {

	if len(a) != len(b) {
		return fmt.Errorf("invalid len. left: %d right: %d", len(a), len(b))
	}

	am := a.Map()
	bm := b.Map()

	for name, h := range am {

		o, ok := bm[name]
		if !ok {
			return fmt.Errorf("'%s': missing", name)
		}

		if h.Hash != o.Hash {
			return fmt.Errorf("'%s': hash mismatch", name)
		}

		if len(h.Params) > 0 {

			if err := cmpH(h.Params, o.Params); err != nil {
				return fmt.Errorf("'%s': invalid param: %w", name, err)
			}
		}
	}

	return nil
}
