package slot

import "testing"

func TestOfficialVectors(t *testing.T) {
	tests := map[string]int{
		"123456789": 12739, // Redis Cluster specification CRC16/XMODEM vector.
		"foo":       12182,
		"bar":       5061,
	}
	for key, want := range tests {
		if got := Slot(key); got != want {
			t.Errorf("Slot(%q)=%d want %d", key, got, want)
		}
	}
	if got := CRC16([]byte("123456789")); got != 0x31C3 {
		t.Fatalf("CRC16=%04x want 31c3", got)
	}
}

func TestHashTag(t *testing.T) {
	tests := map[string]string{
		"user:{1001}:profile": "1001", "order:{1001}:list": "1001",
		"foo{}{bar}": "foo{}{bar}", "foo{{bar}}zap": "{bar", "plain": "plain",
	}
	for key, want := range tests {
		if got := HashTag(key); got != want {
			t.Errorf("HashTag(%q)=%q want %q", key, got, want)
		}
	}
	if Slot("user:{1001}:profile") != Slot("order:{1001}:list") {
		t.Fatal("相同 hash tag 应位于相同槽位")
	}
}
