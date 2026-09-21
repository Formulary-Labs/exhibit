package render

import (
	"encoding/json"
	"fmt"
)

// FilterInternalOnly returns a copy of data with every JSON object that
// contains "internal_only": true removed. When includeInternal is true the
// data is returned unchanged (internal mode — for program managers).
//
// The filter is recursive: it removes any object at any nesting level that
// carries the internal_only flag, and removes array elements that do the same.
// Objects and array elements without the flag are preserved verbatim.
func FilterInternalOnly(data []byte, includeInternal bool) ([]byte, error) {
	if includeInternal {
		return data, nil
	}
	var raw interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("FilterInternalOnly: unmarshal: %w", err)
	}
	filtered := filterValue(raw)
	return json.Marshal(filtered)
}

// filterValue recursively removes objects and array elements that carry
// "internal_only": true.
func filterValue(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		if b, ok := val["internal_only"].(bool); ok && b {
			return nil // signal to caller to exclude this object
		}
		out := make(map[string]interface{}, len(val))
		for k, child := range val {
			filtered := filterValue(child)
			if filtered == nil && child != nil {
				// child was a map with internal_only:true — omit the field
				continue
			}
			out[k] = filtered
		}
		return out
	case []interface{}:
		out := make([]interface{}, 0, len(val))
		for _, item := range val {
			filtered := filterValue(item)
			if filtered == nil && item != nil {
				// array element was internal — drop it
				continue
			}
			out = append(out, filtered)
		}
		return out
	default:
		return v
	}
}
