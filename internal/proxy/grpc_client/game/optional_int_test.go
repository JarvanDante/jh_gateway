package game

import "testing"

func TestOptionalInt32(t *testing.T) {
	data := map[string]interface{}{
		"zero": 0, "zero_str": "0", "one": float64(1), "five_str": "5", "empty": "", "blank": "  ", "nil": nil,
	}
	cases := map[string]int32{
		"missing": -1, "nil": -1, "empty": -1, "blank": -1,
		"zero": 0, "zero_str": 0, "one": 1, "five_str": 5,
	}
	for k, want := range cases {
		if got := optionalInt32(data, k); got != want {
			t.Errorf("%s: got %d want %d", k, got, want)
		}
	}
}
