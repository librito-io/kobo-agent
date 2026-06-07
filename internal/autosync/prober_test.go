package autosync

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSnapshotReady(t *testing.T) {
	cases := []struct {
		name string
		s    Snapshot
		want bool
	}{
		{"all up", Snapshot{Carrier: true, OperstateUp: true, DefaultRoute: true}, true},
		{"no carrier", Snapshot{Carrier: false, OperstateUp: true, DefaultRoute: true}, false},
		{"not operstate up", Snapshot{Carrier: true, OperstateUp: false, DefaultRoute: true}, false},
		{"no default route", Snapshot{Carrier: true, OperstateUp: true, DefaultRoute: false}, false},
		{"all down", Snapshot{}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.s.Ready(); got != c.want {
				t.Fatalf("Ready() = %v, want %v", got, c.want)
			}
		})
	}
}

func TestHasDefaultRoute(t *testing.T) {
	// /proc/net/route is whitespace-separated; col 0 = Iface, col 1 = Destination
	// (hex, little-endian). A default route has Destination 00000000.
	withDefault := "Iface\tDestination\tGateway\tFlags\tRefCnt\tUse\tMetric\tMask\n" +
		"wlan0\t00000000\t0118A8C0\t0003\t0\t0\t0\t00000000\n" +
		"wlan0\t0018A8C0\t00000000\t0001\t0\t0\t0\t00FFFFFF\n"
	subnetOnly := "Iface\tDestination\tGateway\tFlags\tRefCnt\tUse\tMetric\tMask\n" +
		"wlan0\t0018A8C0\t00000000\t0001\t0\t0\t0\t00FFFFFF\n"
	defaultViaEth := "Iface\tDestination\tGateway\tFlags\tRefCnt\tUse\tMetric\tMask\n" +
		"eth0\t00000000\t0118A8C0\t0003\t0\t0\t0\t00000000\n"

	cases := []struct {
		name  string
		route string
		iface string
		want  bool
	}{
		{"default via wlan0", withDefault, "wlan0", true},
		{"subnet only, no default", subnetOnly, "wlan0", false},
		{"default via eth0 not wlan0", defaultViaEth, "wlan0", false},
		{"header only", "Iface\tDestination\tGateway\n", "wlan0", false},
		{"empty", "", "wlan0", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := hasDefaultRoute([]byte(c.route), c.iface); got != c.want {
				t.Fatalf("hasDefaultRoute(%q) = %v, want %v", c.iface, got, c.want)
			}
		})
	}
}

func TestSysfsProber_Probe(t *testing.T) {
	root := t.TempDir()
	sysNet := filepath.Join(root, "sys", "class", "net", "wlan0")
	procNet := filepath.Join(root, "proc", "net")
	if err := os.MkdirAll(sysNet, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(procNet, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(sysNet, "carrier"), "1\n")
	mustWrite(t, filepath.Join(sysNet, "operstate"), "up\n")
	mustWrite(t, filepath.Join(procNet, "route"),
		"Iface\tDestination\tGateway\n wlan0\t00000000\t0118A8C0\n")

	p := newProberAt("wlan0", filepath.Join(root, "sys"), filepath.Join(root, "proc"))
	got := p.Probe()
	if !got.Ready() {
		t.Fatalf("Probe() = %+v, want Ready()", got)
	}

	// Flip carrier down → not ready.
	mustWrite(t, filepath.Join(sysNet, "carrier"), "0\n")
	if p.Probe().Ready() {
		t.Fatal("Probe() should not be Ready with carrier=0")
	}
}

func TestSysfsProber_MissingFiles(t *testing.T) {
	// Absent sysfs (interface gone) → a zero Snapshot, never a panic.
	p := newProberAt("wlan0", t.TempDir(), t.TempDir())
	if p.Probe().Ready() {
		t.Fatal("missing sysfs should yield a not-ready snapshot")
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
