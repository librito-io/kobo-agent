// Package autosync implements the `autosync` subcommand: a udev WiFi-up event
// fires it, and it (under a single-instance lock) waits for connectivity,
// resolves the paired backend, runs the Step-1 sync, and logs the result.
//
// The orchestrator (run.go) is pure over five interfaces — Locker, Config,
// Prober, Syncer, Logger (+ Clock) — so every branch is table-tested on fakes
// with no device and no network. This file is the connectivity edge.
package autosync

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strings"
)

// Snapshot is one reading of an interface's connectivity. Ready() is the pure
// AND of all three signals the spec requires before a sync is attempted.
type Snapshot struct {
	Carrier      bool // /sys/class/net/<iface>/carrier == 1
	OperstateUp  bool // /sys/class/net/<iface>/operstate == "up"
	DefaultRoute bool // a default route (Destination 00000000) via <iface> in /proc/net/route
}

// Ready reports whether the interface is usable for an HTTP sync.
func (s Snapshot) Ready() bool { return s.Carrier && s.OperstateUp && s.DefaultRoute }

// Prober reads the current connectivity snapshot. The sysfs impl is below; the
// orchestrator tests use a scripted fake.
type Prober interface {
	Probe() Snapshot
}

// sysfsProber reads sysfs + /proc. Roots are fields so tests point them at a
// temp tree; production uses /sys and /proc.
type sysfsProber struct {
	iface    string
	sysRoot  string
	procRoot string
}

// NewSysfsProber builds a Prober for iface against the live /sys and /proc.
func NewSysfsProber(iface string) Prober { return newProberAt(iface, "/sys", "/proc") }

func newProberAt(iface, sysRoot, procRoot string) *sysfsProber {
	return &sysfsProber{iface: iface, sysRoot: sysRoot, procRoot: procRoot}
}

func (p *sysfsProber) Probe() Snapshot {
	netDir := filepath.Join(p.sysRoot, "class", "net", p.iface)
	route, _ := os.ReadFile(filepath.Join(p.procRoot, "net", "route"))
	return Snapshot{
		Carrier:      readTrimmed(filepath.Join(netDir, "carrier")) == "1",
		OperstateUp:  readTrimmed(filepath.Join(netDir, "operstate")) == "up",
		DefaultRoute: hasDefaultRoute(route, p.iface),
	}
}

// readTrimmed returns the trimmed file contents, or "" on any read error (a
// missing sysfs file means the signal is simply absent → not-ready).
func readTrimmed(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// hasDefaultRoute reports whether /proc/net/route content has a default route
// (Destination 00000000) on iface. The header row's Destination column is the
// literal "Destination", so it never matches.
func hasDefaultRoute(routeFile []byte, iface string) bool {
	sc := bufio.NewScanner(bytes.NewReader(routeFile))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) >= 2 && f[0] == iface && f[1] == "00000000" {
			return true
		}
	}
	return false
}
