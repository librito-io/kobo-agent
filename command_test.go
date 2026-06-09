package main

import (
	"strings"
	"testing"
)

func TestCommandNames(t *testing.T) {
	got := commandNames(commands)
	want := []string{"pair", "autosync", "watch", "status", "about", "sync-now"}
	if len(got) != len(want) {
		t.Fatalf("commandNames len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("commandNames[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestRenderHelpListsAllCommands(t *testing.T) {
	help := renderHelp(commands)
	for _, c := range commands {
		if !strings.Contains(help, c.name) {
			t.Errorf("renderHelp missing command %q\n---\n%s", c.name, help)
		}
	}
	if !strings.Contains(help, "sync") {
		t.Error("renderHelp should mention the default sync")
	}
}
