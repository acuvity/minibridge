package utils

import (
	"fmt"
	"os"

	"go.acuvity.ai/elemental"
)

// SBOM contains a list of hashes for hashable
// resources.
type SBOM struct {
	Tools   Hashes `json:"tools,omitzero"`
	Prompts Hashes `json:"prompts,omitzero"`
}

func LoadSBOM(path string) (sbom SBOM, err error) {

	data, err := os.ReadFile(path) // #nosec: G304
	if err != nil {
		return sbom, fmt.Errorf("unable to load sbom file at '%s': %w", path, err)
	}

	if err := elemental.Decode(elemental.EncodingTypeJSON, data, &sbom); err != nil {
		return sbom, fmt.Errorf("unable to decode content of sbom file: %w", err)
	}

	return sbom, nil
}

// Hashes are a list of Hash.
type Hashes []Hash

// Matches return nil if both receiver and o
// match, meaning len are identical, and all hashes
// on h match hashes on o.
func (h Hashes) Matches(o Hashes) error {
	return cmpH(h, o)
}

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
