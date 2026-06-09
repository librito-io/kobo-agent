package sync

import (
	"github.com/librito-io/kobo-sync/internal/kobo"
	"github.com/librito-io/kobo-sync/internal/transform"
)

// Options configures one sync run.
type Options struct {
	DBPath  string // path to KoboReader.sqlite
	BaseURL string // Librito API base, e.g. https://librito.io
	Token   string // device token (sk_device_...)
	DryRun  bool   // build + report, do not POST
}

// Outcome reports what a run did, for the CLI to print.
type Outcome struct {
	Read   int                        // raw highlights read from the DB
	Built  int                        // items after mapping/dropping
	Items  []transform.KoboImportItem // the built payload (useful for dry-run print)
	Posted bool                       // whether an HTTP POST was made
	Result ImportResult               // server's {imported, books} when posted
}

// Run reads highlights, maps them to wire items, and (unless DryRun) POSTs the
// full set to the import endpoint. Read → map is always performed; the network
// is only touched on a live run with at least one item.
func Run(opts Options) (Outcome, error) {
	raws, err := kobo.ReadHighlights(opts.DBPath)
	if err != nil {
		return Outcome{}, err
	}

	items := make([]transform.KoboImportItem, 0, len(raws))
	for _, r := range raws {
		if it, ok := transform.BuildItem(r); ok {
			items = append(items, it)
		}
	}

	out := Outcome{Read: len(raws), Built: len(items), Items: items}

	if opts.DryRun {
		return out, nil
	}

	res, err := PostImport(opts.BaseURL, opts.Token, items)
	if err != nil {
		return out, err
	}
	out.Posted = len(items) > 0
	out.Result = res
	return out, nil
}
