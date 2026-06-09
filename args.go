package main

import (
	"flag"
	"fmt"
	"os"
)

// positionalsErr builds the error text for positional args left over after
// flag.Parse, or "" when there are none. Go's flag stops parsing at the first
// non-flag token, so a positional silently drops every flag after it (#33) —
// the message names the token and counts what Parse never reached. Pure for
// table-testing.
func positionalsErr(args []string) string {
	if len(args) == 0 {
		return ""
	}
	if len(args) == 1 {
		return fmt.Sprintf("error: unexpected argument %q", args[0])
	}
	if n := len(args) - 1; n > 1 {
		return fmt.Sprintf("error: unexpected argument %q (the %d args after it were not parsed)", args[0], n)
	}
	return fmt.Sprintf("error: unexpected argument %q (the 1 arg after it was not parsed)", args[0])
}

// rejectPositionals reports whether fs has leftover positional args after
// Parse, printing the error to stderr. No subcommand (nor the default sync)
// takes positionals, so every runner calls this immediately after fs.Parse.
func rejectPositionals(fs *flag.FlagSet) bool {
	msg := positionalsErr(fs.Args())
	if msg == "" {
		return false
	}
	fmt.Fprintln(os.Stderr, msg)
	return true
}
