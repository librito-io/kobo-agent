package autosync

import "github.com/librito-io/kobo-sync/internal/sync"

// Syncer runs the highlight sync. The sync.Run adapter is below; the
// orchestrator tests use a scripted fake.
type Syncer interface {
	// Sync reads highlights and POSTs the full set to baseURL with token,
	// returning the server's (imported, books) counts.
	Sync(baseURL, token string) (imported, books int, err error)
}

// syncRunner adapts the existing internal/sync orchestrator. Step 3 adds no new
// write path — it only triggers the Step-1 sync.
type syncRunner struct{ dbPath string }

// NewSyncer builds a Syncer reading from dbPath (the KoboReader.sqlite path).
func NewSyncer(dbPath string) Syncer { return &syncRunner{dbPath: dbPath} }

func (s *syncRunner) Sync(baseURL, token string) (int, int, error) {
	out, err := sync.Run(sync.Options{DBPath: s.dbPath, BaseURL: baseURL, Token: token, DryRun: false})
	if err != nil {
		return 0, 0, err
	}
	return out.Result.Imported, out.Result.Books, nil
}
