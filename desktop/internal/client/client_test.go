package client

import "testing"

func TestNormalizeBaseURL(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"https://todo.qaer.io", "https://todo.qaer.io/api/v1"},
		{"https://todo.qaer.io/", "https://todo.qaer.io/api/v1"},
		{"https://todo.qaer.io/api/v1", "https://todo.qaer.io/api/v1"},
		{"https://todo.qaer.io/api/v1/", "https://todo.qaer.io/api/v1"},
		{"http://localhost:8099", "http://localhost:8099/api/v1"},
	}
	for _, c := range cases {
		got := NormalizeBaseURL(c.in)
		if got != c.want {
			t.Errorf("NormalizeBaseURL(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
