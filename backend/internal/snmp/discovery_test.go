package snmp

import "testing"

func TestUint64Value(t *testing.T) {
	tests := []struct {
		name string
		in   any
		want uint64
	}{
		{"uint64", uint64(42), 42},
		{"uint32", uint32(42), 42},
		{"int", 42, 42},
		{"string", "42", 42},
		{"bytes", []byte{0x01, 0x02}, 258},
		{"invalid string", "nope", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := uint64Value(tt.in); got != tt.want {
				t.Fatalf("uint64Value(%v) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}
