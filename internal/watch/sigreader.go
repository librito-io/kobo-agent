package watch

import "github.com/librito-io/kobo-agent/internal/kobo"

// fileSigReader reads the signature from a KoboReader.sqlite via the lightweight
// kobo.ReadHighlightSignature (NOT the heavyweight ReadHighlights).
type fileSigReader struct{ dbPath string }

// NewSigReader builds a SigReader over dbPath.
func NewSigReader(dbPath string) SigReader { return &fileSigReader{dbPath: dbPath} }

func (s *fileSigReader) Read() (Signature, error) {
	count, maxDate, err := kobo.ReadHighlightSignature(s.dbPath)
	if err != nil {
		return Signature{}, err
	}
	return Signature{Count: count, MaxDate: maxDate}, nil
}
