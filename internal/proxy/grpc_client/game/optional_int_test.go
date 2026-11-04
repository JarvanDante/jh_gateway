package game

import "testing"

func TestOptionalInt32(t *testing.T) {
	data := map[string]interface{}{
		"zero": 0, "zero_str": "0", "one": float64(1), "five_str": "5",
		"empty": "", "blank": "  ", "nil": nil, "abc": "abc", "space_abc": " abc ",
		"bool": true, "float": 1.5,
	}
	okCases := map[string]int32{
		"missing": -1, "nil": -1, "empty": -1, "blank": -1,
		"zero": 0, "zero_str": 0, "one": 1, "five_str": 5,
	}
	for k, want := range okCases {
		got, err := optionalInt32(data, k)
		if err != nil {
			t.Errorf("%s: unexpected err %v", k, err)
			continue
		}
		if got != want {
			t.Errorf("%s: got %d want %d", k, got, want)
		}
	}
	for _, k := range []string{"abc", "space_abc", "bool", "float"} {
		if _, err := optionalInt32(data, k); err == nil {
			t.Errorf("%s: expected error for non-numeric value", k)
		}
	}
}
