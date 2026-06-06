package pair

import (
	"bytes"
	"strings"
	"testing"
)

func TestGenerateHardwareID_ShapeAndVersionBits(t *testing.T) {
	src := bytes.NewReader(bytes.Repeat([]byte{0xAB}, 16))
	id, err := GenerateHardwareID(src)
	if err != nil {
		t.Fatalf("GenerateHardwareID: %v", err)
	}
	// byte[6]→0x4B (version 4). byte[8]: 0xAB&0x3f|0x80 = 0xAB (top 2 bits already
	// 10), so the variant nibble stays 'a' ∈ {8,9,a,b}; RFC entropy preserved.
	want := "abababab-abab-4bab-abab-ababababab" + "ab"
	if id != want {
		t.Fatalf("id = %q, want %q", id, want)
	}
	if !ValidateHardwareID(id) {
		t.Fatalf("generated id %q failed its own validator", id)
	}
}

func TestGenerateHardwareID_IsLowercase(t *testing.T) {
	src := bytes.NewReader(bytes.Repeat([]byte{0xFF}, 16))
	id, err := GenerateHardwareID(src)
	if err != nil {
		t.Fatalf("GenerateHardwareID: %v", err)
	}
	if id != strings.ToLower(id) {
		t.Fatalf("id %q is not lowercase", id)
	}
}

func TestGenerateHardwareID_ShortReadErrors(t *testing.T) {
	if _, err := GenerateHardwareID(bytes.NewReader([]byte{0x01, 0x02})); err == nil {
		t.Fatal("want error on short read, got nil")
	}
}

func TestValidateHardwareID(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"valid lowercase v4", "abababab-abab-4bab-8bab-abababababab", true},
		{"valid lowercase v4 variant 9", "00000000-0000-4000-9000-000000000000", true},
		{"valid lowercase v4 variant b", "ffffffff-ffff-4fff-bfff-ffffffffffff", true},
		{"uppercase rejected", "ABABABAB-ABAB-4BAB-8BAB-ABABABABABAB", false},
		{"mixed case rejected", "abababab-abab-4bab-8bab-ABABABABABAB", false},
		{"version not 4", "abababab-abab-3bab-8bab-abababababab", false},
		{"variant not 8-b", "abababab-abab-4bab-7bab-abababababab", false},
		{"no hyphens", "abababababab4bab8bababababababab", false},
		{"too short", "abababab-abab-4bab-8bab-abababab", false},
		{"empty", "", false},
		{"non-hex char", "ghghghgh-abab-4bab-8bab-abababababab", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ValidateHardwareID(c.in); got != c.want {
				t.Fatalf("ValidateHardwareID(%q) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}
