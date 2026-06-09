package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestDispatch(t *testing.T) {
	var ranName string
	var ranRest []string
	mk := func(name string, code int) command {
		return command{name, "summary", func(args []string) int { ranName = name; ranRest = args; return code }}
	}
	cmds := []command{mk("pair", 7), mk("watch", 9)}
	var defRest []string
	defRun := func(args []string) int { defRest = args; return 5 }

	t.Run("help -> stdout, exit 0", func(t *testing.T) {
		var out, errb bytes.Buffer
		code := dispatch([]string{"--help"}, "ks", cmds, defRun, &out, &errb)
		if code != 0 {
			t.Errorf("exit = %d, want 0", code)
		}
		if errb.Len() != 0 {
			t.Errorf("stderr = %q, want empty", errb.String())
		}
		if !strings.Contains(out.String(), "pair") {
			t.Errorf("stdout missing command list: %q", out.String())
		}
		if !strings.Contains(out.String(), "ks") {
			t.Errorf("stdout should name the program as invoked (ks): %q", out.String())
		}
	})

	t.Run("unknown -> stderr, exit 2", func(t *testing.T) {
		var out, errb bytes.Buffer
		code := dispatch([]string{"nope"}, "ks", cmds, defRun, &out, &errb)
		if code != 2 {
			t.Errorf("exit = %d, want 2", code)
		}
		if out.Len() != 0 {
			t.Errorf("stdout = %q, want empty", out.String())
		}
		if !strings.Contains(errb.String(), `unknown command "nope"`) {
			t.Errorf("stderr = %q, want unknown-command error", errb.String())
		}
		if !strings.Contains(errb.String(), "see 'ks --help'") {
			t.Errorf("stderr should suggest the invoked program name: %q", errb.String())
		}
	})

	t.Run("subcommand runs with rest", func(t *testing.T) {
		ranName, ranRest = "", nil
		var out, errb bytes.Buffer
		code := dispatch([]string{"watch", "--probe"}, "ks", cmds, defRun, &out, &errb)
		if code != 9 {
			t.Errorf("exit = %d, want 9", code)
		}
		if ranName != "watch" {
			t.Errorf("ran %q, want watch", ranName)
		}
		if len(ranRest) != 1 || ranRest[0] != "--probe" {
			t.Errorf("rest = %v, want [--probe]", ranRest)
		}
	})

	t.Run("default sync for flags-first", func(t *testing.T) {
		defRest = nil
		var out, errb bytes.Buffer
		code := dispatch([]string{"--dry-run"}, "ks", cmds, defRun, &out, &errb)
		if code != 5 {
			t.Errorf("exit = %d, want 5", code)
		}
		if len(defRest) != 1 || defRest[0] != "--dry-run" {
			t.Errorf("defaultRun rest = %v, want [--dry-run]", defRest)
		}
	})

	t.Run("default sync for no args", func(t *testing.T) {
		var out, errb bytes.Buffer
		code := dispatch(nil, "ks", cmds, defRun, &out, &errb)
		if code != 5 {
			t.Errorf("exit = %d, want 5", code)
		}
	})
}
