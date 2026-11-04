package manage

import (
	"encoding/json"
	"testing"
)

func TestOptionalInt32Ptr(t *testing.T) {
	ok := []struct {
		v    interface{}
		want *int32
	}{
		{nil, nil}, {"", nil}, {"  ", nil},
		{"0", i32(0)}, {0, i32(0)}, {float64(0), i32(0)}, {"2", i32(2)}, {" 3 ", i32(3)},
		{float64(4), i32(4)}, {int64(1), i32(1)}, {json.Number("2"), i32(2)},
	}
	for _, c := range ok {
		got, err := optionalInt32Ptr(map[string]interface{}{"decimals": c.v}, "decimals")
		if err != nil {
			t.Errorf("%#v: unexpected err %v", c.v, err)
			continue
		}
		if (got == nil) != (c.want == nil) || (got != nil && *got != *c.want) {
			t.Errorf("%#v: got %v want %v", c.v, got, c.want)
		}
	}
	if got, err := optionalInt32Ptr(map[string]interface{}{}, "decimals"); got != nil || err != nil {
		t.Errorf("missing key: %v %v", got, err)
	}
	bad := []interface{}{"abc", "1.5", float64(1.5), true, []int{1}, json.Number("1.2"), "99999999999", int64(1 << 40)}
	for _, v := range bad {
		if _, err := optionalInt32Ptr(map[string]interface{}{"symbol_position": v}, "symbol_position"); err == nil {
			t.Errorf("%#v: expected error", v)
		}
	}
}

func i32(v int32) *int32 { return &v }
