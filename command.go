package main

import (
	"fmt"
	"strings"
)

// progName is the canonical program name used in help + error output.
const progName = "librito-kobo-sync"

// command is one named subcommand: its name, a one-line summary for help, and
// the function that runs it (returns a process exit code).
type command struct {
	name    string
	summary string
	run     func(args []string) int
}

// commands is the registry — the single source of truth for both dispatch and
// the rendered help text. The default (nameless) action is a sync; see main().
var commands = []command{
	{"pair", "pair this device (writes hardware-id + token)", runPair},
	{"autosync", "triggered sync for the udev WiFi-up hook (token + url from files)", runAutosync},
	{"watch", "resident daemon: sync on a new highlight while connected", runWatch},
	{"status", "print the last-sync status line", runStatus},
	{"about", "print pairing info + version", runAbout},
	{"sync-now", "one-shot sync with on-screen feedback (for NickelMenu)", runSyncNow},
}

// commandNames returns just the names, for route's known-set.
func commandNames(cmds []command) []string {
	names := make([]string, len(cmds))
	for i, c := range cmds {
		names[i] = c.name
	}
	return names
}

// renderHelp builds the top-level help text from the command registry, so the
// list never drifts from dispatch. prog is the program name as invoked.
func renderHelp(prog string, cmds []command) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s — sync Kobo highlights to Librito\n\n", prog)
	fmt.Fprintf(&b, "Usage:\n")
	fmt.Fprintf(&b, "  %s [flags]            run a sync (default; no subcommand)\n", prog)
	fmt.Fprintf(&b, "  %s <command> [args]\n\n", prog)
	fmt.Fprintf(&b, "Commands:\n")
	for _, c := range cmds {
		fmt.Fprintf(&b, "  %-9s %s\n", c.name, c.summary)
	}
	// The default-sync flag list is hand-maintained (the five flags are stable;
	// they live on runSync's FlagSet and are documented in the README).
	fmt.Fprintf(&b, "\nWith no command, %s performs a sync. Default-sync flags:\n", prog)
	fmt.Fprintf(&b, "  --db --url --token --dir --dry-run   (see README for details)\n")
	return b.String()
}
