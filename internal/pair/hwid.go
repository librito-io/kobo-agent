// Package pair implements the `pair` subcommand: an unpaired Kobo obtains a
// device token via the librito-io/web pairing API, displaying the code and
// polling for the claim through a single live-updating NickelDBus dialog.
package pair

import (
	"fmt"
	"io"
)

const hwidLen = 36 // canonical UUID: 8-4-4-4-12 with four hyphens

// GenerateHardwareID reads 16 random bytes from r and formats them as a
// lowercase canonical UUID v4 (RFC 4122: version nibble forced to 4, variant
// bits to 10). Pass crypto/rand.Reader in production; tests inject fixed bytes.
//
// Lowercase is load-bearing: the web (user_id, hardware_id) UNIQUE is
// case-sensitive text, so a mixed-case resend would mint a phantom device.
func GenerateHardwareID(r io.Reader) (string, error) {
	var b [16]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4: force top nibble to 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10xx: force top 2 bits, keep RFC entropy
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

// ValidateHardwareID reports whether s is a lowercase canonical UUID v4:
// 8-4-4-4-12 lowercase hex, version nibble '4', variant nibble in [8,b].
func ValidateHardwareID(s string) bool {
	if len(s) != hwidLen {
		return false
	}
	for i, c := range s {
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		case 14: // version
			if c != '4' {
				return false
			}
		case 19: // variant: 8,9,a,b
			if c != '8' && c != '9' && c != 'a' && c != 'b' {
				return false
			}
		default:
			if !isLowerHex(c) {
				return false
			}
		}
	}
	return true
}

func isLowerHex(c rune) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')
}
