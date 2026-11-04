package manage

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// optionalInt32Ptr parses an optional integer field. Missing, null or blank
// values return nil (field not modified); explicit 0 is kept. Non-numeric or
// non-integer values return an error so the caller can answer 400 before
// calling manage_service.
func optionalInt32Ptr(data map[string]interface{}, key string) (*int32, error) {
	v, ok := data[key]
	if !ok || v == nil {
		return nil, nil
	}
	var n int64
	switch x := v.(type) {
	case string:
		s := strings.TrimSpace(x)
		if s == "" {
			return nil, nil
		}
		p, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%s 必须是整数", key)
		}
		n = p
	case float64:
		if math.IsNaN(x) || math.IsInf(x, 0) || x != math.Trunc(x) || x < math.MinInt32 || x > math.MaxInt32 {
			return nil, fmt.Errorf("%s 必须是整数", key)
		}
		n = int64(x)
	case float32:
		f := float64(x)
		if f != math.Trunc(f) || f < math.MinInt32 || f > math.MaxInt32 {
			return nil, fmt.Errorf("%s 必须是整数", key)
		}
		n = int64(f)
	case int:
		n = int64(x)
	case int32:
		n = int64(x)
	case int64:
		n = x
	case json.Number:
		p, err := x.Int64()
		if err != nil {
			return nil, fmt.Errorf("%s 必须是整数", key)
		}
		n = p
	default:
		return nil, fmt.Errorf("%s 必须是整数", key)
	}
	if n < math.MinInt32 || n > math.MaxInt32 {
		return nil, fmt.Errorf("%s 超出范围", key)
	}
	r := int32(n)
	return &r, nil
}
