package manage

import (
	"encoding/json"
	"math"
	"testing"
)

func TestOptionalInt32PtrEdges(t *testing.T) {
	ok := []struct {
		v    interface{}
		want int32
	}{
		{"0", 0}, {float64(0), 0}, {" 2 ", 2}, {"-1", -1}, {json.Number("4"), 4},
		{float64(math.MaxInt32), math.MaxInt32}, {int64(math.MinInt32), math.MinInt32}, {int(3), 3},
	}
	for _, c := range ok {
		p, err := optionalInt32Ptr(map[string]interface{}{"decimals": c.v}, "decimals")
		if err != nil || p == nil || *p != c.want {
			t.Errorf("%#v: p=%v err=%v", c.v, p, err)
		}
	}
	for _, v := range []interface{}{nil, "", "   "} {
		p, err := optionalInt32Ptr(map[string]interface{}{"decimals": v}, "decimals")
		if err != nil || p != nil {
			t.Errorf("%#v should mean unchanged: p=%v err=%v", v, p, err)
		}
	}
	if p, err := optionalInt32Ptr(map[string]interface{}{}, "decimals"); err != nil || p != nil {
		t.Errorf("missing key: p=%v err=%v", p, err)
	}
	for _, v := range []interface{}{
		"abc", "1.5", "2e1", "0x10", true, false, float64(1.5), float32(2.5), json.Number("1.5"),
		float64(math.MaxInt32) + 1, "2147483648", int64(math.MaxInt32) + 1, int64(math.MinInt32) - 1,
		math.NaN(), math.Inf(1), []interface{}{1}, map[string]interface{}{"a": 1},
	} {
		if p, err := optionalInt32Ptr(map[string]interface{}{"decimals": v}, "decimals"); err == nil {
			t.Errorf("%#v should be rejected, got %v", v, *p)
		}
	}
}
