package pair

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectModel(t *testing.T) {
	dir := t.TempDir()

	// Real Libra Colour version line → marketing name.
	vp := filepath.Join(dir, "version")
	if err := os.WriteFile(vp, []byte("N428520323250,4.9.77,4.45.23697,4.9.77,4.9.77,00000000-0000-0000-0000-000000000390\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := DetectModel(vp); got != "Kobo Libra Colour" {
		t.Fatalf("DetectModel(real) = %q, want Kobo Libra Colour", got)
	}

	// Missing file → bare "Kobo" fallback, no error.
	if got := DetectModel(filepath.Join(dir, "nope")); got != "Kobo" {
		t.Fatalf("DetectModel(missing) = %q, want Kobo", got)
	}

	// Unparseable contents → "Kobo" fallback.
	bad := filepath.Join(dir, "bad")
	if err := os.WriteFile(bad, []byte("garbage\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := DetectModel(bad); got != "Kobo" {
		t.Fatalf("DetectModel(garbage) = %q, want Kobo", got)
	}

	// Unknown but well-formed id → legible fallback.
	unk := filepath.Join(dir, "unk")
	if err := os.WriteFile(unk, []byte("S,K,F,K,K,00000000-0000-0000-0000-000000000999\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := DetectModel(unk); got != "Kobo (model 999)" {
		t.Fatalf("DetectModel(unknown) = %q, want Kobo (model 999)", got)
	}
}

func TestParseModelID(t *testing.T) {
	cases := []struct {
		name    string
		version string
		wantID  string
		wantOK  bool
	}{
		{
			// Real Libra Colour dump (2026-06-06): 6 CSV fields, the 6th is a
			// zero-padded UUID whose trailing decimal is the Kobo device id.
			name:    "libra colour real dump",
			version: "N428520323250,4.9.77,4.45.23697,4.9.77,4.9.77,00000000-0000-0000-0000-000000000390",
			wantID:  "390",
			wantOK:  true,
		},
		{"trailing newline tolerated", "S,K,F,K,K,00000000-0000-0000-0000-000000000310\n", "310", true},
		{"clara bw id", "S,K,F,K,K,00000000-0000-0000-0000-000000000376", "376", true},
		{"id is exactly the segment (no leading zeros)", "S,K,F,K,K,00000000-0000-0000-0000-000000000017", "17", true},
		{"all-zero segment → id 0", "S,K,F,K,K,00000000-0000-0000-0000-000000000000", "0", true},
		{"too few fields", "S,K,F,K,K", "", false},
		{"sixth field not a uuid", "S,K,F,K,K,garbage", "", false},
		{"sixth field empty", "S,K,F,K,K,", "", false},
		{"empty input", "", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			id, ok := parseModelID(c.version)
			if ok != c.wantOK || id != c.wantID {
				t.Fatalf("parseModelID(%q) = (%q,%v), want (%q,%v)", c.version, id, ok, c.wantID, c.wantOK)
			}
		})
	}
}

func TestKoboModelName(t *testing.T) {
	cases := []struct {
		id   string
		want string
	}{
		{"390", "Kobo Libra Colour"},
		{"393", "Kobo Clara Colour"},
		{"387", "Kobo Clara 2E"},
		{"376", "Kobo Clara HD"},
		{"310", "Kobo Touch"},
		// Unknown id → legible, stable fallback (never silently mislabels as a
		// known model). Web maps codes later per the pairing contract.
		{"99999", "Kobo (model 99999)"},
		{"", "Kobo"},
	}
	for _, c := range cases {
		t.Run(c.id, func(t *testing.T) {
			if got := koboModelName(c.id); got != c.want {
				t.Fatalf("koboModelName(%q) = %q, want %q", c.id, got, c.want)
			}
		})
	}
}
