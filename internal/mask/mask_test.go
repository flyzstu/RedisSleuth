package mask

import "testing"

func TestKey(t *testing.T) {
	tests := map[string]string{
		"user:100001:profile": "user:******:profile",
		"order:{1001}:list":   "order:{******}:list",
		"product:abc:v2":      "product:abc:v2",
	}
	for in, want := range tests {
		if got := Key(in); got != want {
			t.Errorf("Key(%q)=%q want %q", in, got, want)
		}
	}
}
