package sqlite

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
)

// FileDSN escapes a filesystem path and applies PRAGMAs to every connection,
// including connections database/sql creates after the pool has been opened.
func FileDSN(path string, pragmas ...string) string {
	path = filepath.ToSlash(path)
	if filepath.IsAbs(path) && !strings.HasPrefix(path, "/") {
		path = "/" + path // Windows drive letter in a file URI.
	}
	q := url.Values{}
	for _, pragma := range pragmas {
		q.Add("_pragma", pragma)
	}
	// Prefix the escaped path ourselves: URL{Scheme: "file", Path: "./x"}
	// would serialize with an authority (file://./x), which SQLite rejects.
	return "file:" + (&url.URL{Path: path, RawQuery: q.Encode()}).String()
}

func driverDSN(dsn string) (string, error) {
	path, query, _ := strings.Cut(dsn, "?")
	q, err := url.ParseQuery(query)
	if err != nil {
		return "", fmt.Errorf("invalid SQLite DSN: %w", err)
	}
	if q.Has("_db_key") {
		return "", fmt.Errorf("SQLite file encryption is not supported; remove _db_key and use a standard SQLite database")
	}
	// time.Time.String (the driver default) cannot be used by SQLite's
	// datetime()/strftime(). Preserve SQL-compatible timestamps and UTC offsets.
	if !q.Has("_time_format") {
		q.Set("_time_format", "sqlite")
	}
	if !q.Has("_busy_timeout") && !q.Has("_timeout") {
		q.Set("_busy_timeout", "5000")
	}
	return path + "?" + q.Encode(), nil
}
