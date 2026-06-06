package pair

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestStore_LoadOrCreateHardwareID_GeneratesThenReuses(t *testing.T) {
	dir := t.TempDir()
	rnd := bytes.NewReader(bytes.Repeat([]byte{0x11}, 16))
	s := NewFileStore(dir, rnd)

	id1, err := s.LoadOrCreateHardwareID()
	if err != nil {
		t.Fatalf("first LoadOrCreate: %v", err)
	}
	if !ValidateHardwareID(id1) {
		t.Fatalf("generated id %q not valid", id1)
	}
	// File written.
	onDisk, err := os.ReadFile(filepath.Join(dir, "hardware-id"))
	if err != nil {
		t.Fatalf("hardware-id not written: %v", err)
	}
	if string(bytes.TrimSpace(onDisk)) != id1 {
		t.Fatalf("file %q != returned %q", onDisk, id1)
	}
	// Second call (even with a DIFFERENT rand source) returns the SAME id.
	s2 := NewFileStore(dir, bytes.NewReader(bytes.Repeat([]byte{0x99}, 16)))
	id2, err := s2.LoadOrCreateHardwareID()
	if err != nil {
		t.Fatalf("second LoadOrCreate: %v", err)
	}
	if id2 != id1 {
		t.Fatalf("id changed across runs: %q vs %q", id1, id2)
	}
}

func TestStore_LoadOrCreateHardwareID_RejectsCorruptFile(t *testing.T) {
	dir := t.TempDir()
	// A pre-existing file with an uppercase/invalid id must NOT be silently
	// reused (it would mint a phantom device); regenerate instead.
	if err := os.WriteFile(filepath.Join(dir, "hardware-id"), []byte("NOT-A-UUID\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	rnd := bytes.NewReader(bytes.Repeat([]byte{0x22}, 16))
	id, err := NewFileStore(dir, rnd).LoadOrCreateHardwareID()
	if err != nil {
		t.Fatalf("LoadOrCreate over corrupt: %v", err)
	}
	if !ValidateHardwareID(id) {
		t.Fatalf("did not regenerate a valid id, got %q", id)
	}
}

func TestStore_WriteToken_OverwritesAndPermissions(t *testing.T) {
	dir := t.TempDir()
	s := NewFileStore(dir, bytes.NewReader(bytes.Repeat([]byte{0x33}, 16)))

	if err := s.WriteToken("sk_device_first"); err != nil {
		t.Fatalf("WriteToken: %v", err)
	}
	if err := s.WriteToken("sk_device_second"); err != nil { // re-pair overwrites
		t.Fatalf("WriteToken overwrite: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "token"))
	if err != nil {
		t.Fatalf("token not written: %v", err)
	}
	if string(bytes.TrimSpace(b)) != "sk_device_second" {
		t.Fatalf("token = %q, want sk_device_second", b)
	}
	fi, _ := os.Stat(filepath.Join(dir, "token"))
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("token perm = %v, want 0600", fi.Mode().Perm())
	}
}
