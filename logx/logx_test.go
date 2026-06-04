package logx

import "testing"

func TestMaskSecret(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty", in: "", want: ""},
		{name: "short", in: "abc", want: "***"},
		{name: "six_chars", in: "abcdef", want: "***"},
		{name: "long", in: "sk-123456789", want: "***456789"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := MaskSecret(tc.in); got != tc.want {
				t.Fatalf("MaskSecret(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
