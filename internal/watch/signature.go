package watch

// Signature is the lightweight fingerprint of the visible highlight set: the
// count and the max DateCreated. It is the watch daemon's gate against the chatty
// WAL (read-position/settings writes do not change it).
type Signature struct {
	Count   int
	MaxDate string
}

// grew reports whether cur represents new highlight activity vs prev: either the
// count rose OR the max DateCreated advanced. Max-date is the robust signal — it
// catches a delete-then-add that leaves Count flat. The comparison is bytewise,
// correct for the uniform naive-UTC ISO format Kobo writes (spec _Assumptions_).
func grew(prev, cur Signature) bool {
	return cur.Count > prev.Count || cur.MaxDate > prev.MaxDate
}
