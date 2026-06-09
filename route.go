package main

import "strings"

// routeKind is the dispatcher's decision for a given argv.
type routeKind int

const (
	routeDefault    routeKind = iota // run the default sync with rest
	routeHelp                        // print top-level help
	routeSubcommand                  // run the named subcommand with rest
	routeUnknown                     // first token is an unknown command
)

// routeResult is the outcome of route: a kind plus its payload.
type routeResult struct {
	kind routeKind
	name string   // subcommand name (routeSubcommand) or offending token (routeUnknown)
	rest []string // args passed through to the runner (routeDefault / routeSubcommand)
}

// route decides what argv means, given the set of known subcommand names. Pure
// and total so dispatch can be table-tested. Precedence:
//  1. no args                        -> default sync
//  2. a help spelling as first token -> help (BEFORE the flag check, so
//     "--help"/"-help" show the top-level list, not sync's flags)
//  3. a known subcommand             -> that subcommand, rest = args[1:]
//  4. a real flag (starts "-", len>1) -> default sync, rest = args
//  5. any other non-flag word        -> unknown command
//
// A bare "-" is deliberately NOT treated as a flag: Go's flag package stops
// parsing at a lone "-" (it is not a flag), so routing it to the default sync
// would silently drop every flag after it — the exact defect this dispatcher
// exists to prevent. It falls through to rule 5 (unknown) instead.
func route(args, known []string) routeResult {
	if len(args) == 0 {
		return routeResult{kind: routeDefault}
	}
	first := args[0]
	if isHelpToken(first) {
		return routeResult{kind: routeHelp}
	}
	for _, k := range known {
		if first == k {
			return routeResult{kind: routeSubcommand, name: first, rest: args[1:]}
		}
	}
	if len(first) > 1 && strings.HasPrefix(first, "-") {
		return routeResult{kind: routeDefault, rest: args}
	}
	return routeResult{kind: routeUnknown, name: first}
}

// isHelpToken reports whether s is a top-level help spelling. Go's flag package
// treats single- and double-dash as the same flag, so both "-help" and "--help"
// are covered to keep top-level behavior consistent.
func isHelpToken(s string) bool {
	switch s {
	case "help", "-h", "--h", "-help", "--help":
		return true
	}
	return false
}
