package models

import "testing"

func TestPriorityRankFromString(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"high", 1},
		{"HIGH", 1},
		{" urgent ", 1},
		{"normal", 2},
		{"", 2},
		{"low", 3},
	}
	for _, tc := range cases {
		if got := PriorityRankFromString(tc.in); got != tc.want {
			t.Errorf("PriorityRankFromString(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}
