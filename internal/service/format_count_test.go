package service

import "testing"

func TestFormatCount(t *testing.T) {
	cases := []struct {
		in   interface{}
		want string
	}{
		{0, "0"}, {999, "999"}, {1000, "1,000"}, {3412, "3,412"}, {int64(1234567), "1,234,567"},
		{float64(41200), "41,200"}, {float64(2.5), "2.5"}, {-12345, "-12,345"}, {nil, ""}, {"7", "7"},
	}
	for _, c := range cases {
		if got := formatCount(c.in); got != c.want {
			t.Errorf("formatCount(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}
