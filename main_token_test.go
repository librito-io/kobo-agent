package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveToken_Precedence(t *testing.T) {
	dir := t.TempDir()
	tokFile := filepath.Join(dir, "token")
	if err := os.WriteFile(tokFile, []byte("sk_device_fromfile\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	// flag wins over env and file
	if got := resolveToken("sk_flag", "sk_env", tokFile); got != "sk_flag" {
		t.Fatalf("flag precedence: got %q", got)
	}
	// env wins over file
	if got := resolveToken("", "sk_env", tokFile); got != "sk_env" {
		t.Fatalf("env precedence: got %q", got)
	}
	// file used when neither flag nor env
	if got := resolveToken("", "", tokFile); got != "sk_device_fromfile" {
		t.Fatalf("file precedence: got %q", got)
	}
	// nothing available → empty
	if got := resolveToken("", "", filepath.Join(dir, "missing")); got != "" {
		t.Fatalf("no token: got %q", got)
	}
}
