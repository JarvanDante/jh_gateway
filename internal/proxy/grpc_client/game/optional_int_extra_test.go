package game

import (
	"encoding/json"
	"testing"
)

func TestOptionalInt32_ExtraTypes(t *testing.T) {
	data := map[string]interface{}{
		"i32": int32(-1), "i64": int64(7), "jn": json.Number("12"),
		"jn_bad": json.Number("1.5"), "big": int64(1 << 40),
	}
	ok := map[string]int32{"i32": -1, "i64": 7, "jn": 12}
	for k, want := range ok {
		got, err := optionalInt32(data, k)
		if err != nil || got != want {
			t.Errorf("%s: got %d err=%v want %d", k, got, err, want)
		}
	}
	for _, k := range []string{"jn_bad", "big"} {
		if _, err := optionalInt32(data, k); err == nil {
			t.Errorf("%s: expected error", k)
		}
	}
}
