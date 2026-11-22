// Package bodyinspect reads bounded prefixes while retaining the upstream stream.
package bodyinspect

import (
	"bytes"
	"io"
	"net/http"
	"time"
)

const maxPrefixBytes = 32 << 20

type replay struct {
	io.Reader
	original io.ReadCloser
}

func (r *replay) Close() error { return r.original.Close() }

type readError struct{ err error }

func (e *readError) Read([]byte) (int, error) {
	if e.err == nil {
		return 0, io.EOF
	}
	err := e.err
	e.err = nil
	return 0, err
}

// ReadPrefix reads limit+1 bytes to detect truncation, then restores every byte,
// the unread tail, the original read error, and the original Close operation.
func ReadPrefix(r *http.Request, limit int64) ([]byte, error) {
	return readPrefix(r, limit, nil)
}

func readPrefix(r *http.Request, limit int64, lastRead *time.Time) ([]byte, error) {
	if r.Body == nil || r.Body == http.NoBody {
		return nil, nil
	}
	if limit < 0 {
		limit = 0
	}
	if limit > maxPrefixBytes {
		limit = maxPrefixBytes
	}
	original := r.Body
	var reader io.Reader = original
	if lastRead != nil {
		reader = progressReader{reader, lastRead}
	}
	data, err := io.ReadAll(io.LimitReader(reader, limit+1))
	readers := []io.Reader{bytes.NewReader(data)}
	if err != nil {
		readers = append(readers, &readError{err: err})
	}
	readers = append(readers, original)
	r.Body = &replay{Reader: io.MultiReader(readers...), original: original}
	return data, err
}

type progressReader struct {
	reader   io.Reader
	lastRead *time.Time
}

func (r progressReader) Read(buf []byte) (int, error) {
	n, err := r.reader.Read(buf)
	if n > 0 {
		*r.lastRead = time.Now()
	}
	return n, err
}
