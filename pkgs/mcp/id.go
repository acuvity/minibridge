package mcp

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
)

// RelatedIDs returns true if the two given
// ID as any are equal.
// Slow clap on spec that basically says
// MUST SHOULD BE AN INT. Or a string. whatever...
func RelatedIDs(a any, b any) bool {

	if sa, ok := a.(string); ok {
		sb, ok := b.(string)
		return ok && sa == sb
	}

	if isNumeric(a) && isNumeric(b) {
		ia, oka := extractInt64(a)
		ib, okb := extractInt64(b)
		return oka && okb && ia == ib
	}

	return reflect.TypeOf(a) == reflect.TypeOf(b) && reflect.DeepEqual(a, b)
}

func normalizeID(id any) string {
	switch v := id.(type) {
	case string:
		return v
	case int:
		return fmt.Sprintf("%d", v)
	case int64:
		return fmt.Sprintf("%d", v)
	case uint64:
		return fmt.Sprintf("%d", v)
	case float64:
		return fmt.Sprintf("%.0f", v)
	case json.Number:
		return v.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}

func isNumeric(v any) bool {
	switch v.(type) {
	case int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64, json.Number:
		return true
	default:
		return false
	}
}

func extractInt64(v any) (int64, bool) {
	switch val := v.(type) {
	case int:
		return int64(val), true
	case int64:
		return val, true
	case int32:
		return int64(val), true
	case int16:
		return int64(val), true
	case int8:
		return int64(val), true
	case uint:
		if val <= math.MaxInt64 {
			return int64(val), true
		}
		return 0, false
	case uint64:
		if val <= math.MaxInt64 {
			return int64(val), true
		}
		return 0, false
	case uint32:
		return int64(val), true
	case float64:
		if val == float64(int64(val)) {
			return int64(val), true
		}
		return 0, false
	case float32:
		f := float64(val)
		if f == float64(int64(f)) {
			return int64(f), true
		}
		return 0, false
	case json.Number:
		if i, err := val.Int64(); err == nil {
			return i, true
		}
		return 0, false
	default:
		return 0, false
	}
}
