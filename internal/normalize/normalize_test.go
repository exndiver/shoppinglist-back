package normalize

import "testing"

func TestName(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"  ", ""},
		{"Milk", "milk"},
		{"  Whole   Milk  ", "whole milk"},
		{"COCA-COLA", "coca-cola"},
		{"Молоко", "молоко"},
		{"  a  b  c  ", "a b c"},
		{"already lowercase", "already lowercase"},
	}
	for _, tc := range cases {
		if got := Name(tc.in); got != tc.want {
			t.Errorf("Name(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
