package main

import (
	"fmt"
	"io"
)

// dispatch routes argv and runs the result, returning a process exit code. The
// program name (for help/error text), the registry, the default-sync runner,
// and the output streams are parameters so dispatch is fully testable (no
// os.Exit, no global I/O). route() decides; this only acts.
func dispatch(args []string, prog string, cmds []command, defaultRun func([]string) int, stdout, stderr io.Writer) int {
	r := route(args, commandNames(cmds))
	switch r.kind {
	case routeHelp:
		fmt.Fprint(stdout, renderHelp(prog, cmds))
		return 0
	case routeUnknown:
		fmt.Fprintf(stderr, "error: unknown command %q\nsee '%s --help' for the list of commands\n", r.name, prog)
		return 2
	case routeSubcommand:
		for _, c := range cmds {
			if c.name == r.name {
				return c.run(r.rest)
			}
		}
		// route only returns names drawn from cmds, so this is unreachable — but
		// be loud rather than exit 2 silently if that invariant ever breaks.
		fmt.Fprintf(stderr, "internal error: routed to unknown command %q\n", r.name)
		return 2
	default: // routeDefault
		return defaultRun(r.rest)
	}
}
