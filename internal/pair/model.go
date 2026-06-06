package pair

import (
	"os"
	"strings"
)

// DefaultVersionPath is where Nickel writes the device version line.
const DefaultVersionPath = "/mnt/onboard/.kobo/version"

// DetectModel reads the Kobo version file and returns a human-legible model
// name for devices.model. Unreadable file or unrecognized id degrades to a
// stable fallback ("Kobo" / "Kobo (model <id>)") — it never errors and never
// silently mislabels, so the pairing request always carries some legible model.
func DetectModel(versionPath string) string {
	b, err := os.ReadFile(versionPath)
	if err != nil {
		return koboModelName("") // "Kobo"
	}
	id, ok := parseModelID(string(b))
	if !ok {
		return koboModelName("")
	}
	return koboModelName(id)
}

// parseModelID extracts the Kobo device id from a /mnt/onboard/.kobo/version
// line. Verified format (Libra Colour, 2026-06-06):
//
//	SERIAL,KERNEL,FIRMWARE,KERNEL,KERNEL,00000000-0000-0000-0000-0000000003XX
//
// Six comma-separated fields; the sixth is a zero-padded UUID whose final
// segment's trailing decimal is the device id (390 = Libra Colour). Returns
// (id, false) when the line doesn't match this shape — the caller falls back to
// a generic model string rather than guessing.
func parseModelID(version string) (string, bool) {
	fields := strings.Split(strings.TrimSpace(version), ",")
	if len(fields) != 6 {
		return "", false
	}
	uuid := fields[5]
	segs := strings.Split(uuid, "-")
	if len(segs) != 5 || len(segs[4]) != 12 {
		return "", false
	}
	last := segs[4]
	if !allDigits(last) {
		return "", false
	}
	// Strip leading zeros; an all-zero segment is id "0".
	id := strings.TrimLeft(last, "0")
	if id == "" {
		id = "0"
	}
	return id, true
}

// koboModelName maps a Kobo device id to a stable, human-legible marketing name
// for devices.model (debug/telemetry, web-mapped later). An unrecognized id
// returns "Kobo (model <id>)" — legible and stable, never a wrong known name.
// An empty id (detection failed entirely) returns the bare "Kobo".
//
// Ids are the Kobo community device table; extend as new models are verified.
func koboModelName(id string) string {
	if id == "" {
		return "Kobo"
	}
	if name, ok := koboModels[id]; ok {
		return name
	}
	return "Kobo (model " + id + ")"
}

// allDigits reports whether s is non-empty and all ASCII digits.
func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

var koboModels = map[string]string{
	"310": "Kobo Touch",
	"320": "Kobo Touch",
	"330": "Kobo Glo",
	"340": "Kobo Aura HD",
	"350": "Kobo Aura",
	"360": "Kobo Aura H2O",
	"370": "Kobo Glo HD",
	"371": "Kobo Touch 2.0",
	"372": "Kobo Aura ONE",
	"373": "Kobo Aura Edition 2",
	"374": "Kobo Aura H2O Edition 2",
	"375": "Kobo Aura Edition 2",
	"376": "Kobo Clara HD",
	"377": "Kobo Forma",
	"378": "Kobo Aura H2O Edition 2",
	"379": "Kobo Libra H2O",
	"380": "Kobo Nia",
	"381": "Kobo Sage",
	"382": "Kobo Elipsa",
	"383": "Kobo Libra 2",
	"384": "Kobo Clara 2E",
	"387": "Kobo Clara 2E",
	"388": "Kobo Sage",
	"389": "Kobo Elipsa 2E",
	"390": "Kobo Libra Colour",
	"391": "Kobo Clara Colour",
	"393": "Kobo Clara Colour",
}
