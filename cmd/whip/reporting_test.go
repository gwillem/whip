package main

import "testing"

func TestUseTUI(t *testing.T) {
	cases := []struct {
		name      string
		verbosity int
		isTTY     bool
		want      bool
	}{
		{"tty, no verbose -> tui", 0, true, true},
		{"no tty, no verbose -> fallback", 0, false, false},
		{"tty, verbose -> plain", 1, true, false},
		{"no tty, verbose -> plain", 1, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := useTUI(c.verbosity, c.isTTY); got != c.want {
				t.Errorf("useTUI(%d, %v) = %v, want %v", c.verbosity, c.isTTY, got, c.want)
			}
		})
	}
}
