// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package driver

import (
	"fmt"
	"github.com/go-mysql-org/go-mysql/mysql"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var charsetPattern = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

type Config struct {
	Connector Connector
	Loc       *time.Location
	DSN       string
}

func NewConfig() *Config { return &Config{Loc: time.UTC} }
func ParseConfig(dsn string) (*Config, error) {
	connector, err := ParseDSN(dsn)
	if err != nil {
		return nil, err
	}
	loc := time.UTC
	if name := connector.Params.Get("loc"); name != "" {
		loc, err = time.LoadLocation(name)
		if err != nil {
			return nil, err
		}
	}
	return &Config{Connector: connector, Loc: loc, DSN: dsn}, nil
}
func (c *Config) FormatDSN() string {
	if c.DSN != "" {
		return c.DSN
	}
	u := url.URL{Scheme: "mysql", User: url.UserPassword(c.Connector.User, c.Connector.Password), Host: c.Connector.Addr, Path: "/" + c.Connector.DB, RawQuery: c.Connector.Params.Encode()}
	return u.String()
}

// Parse the legacy transport delimiter before URL parsing, so credentials that
// contain @, /, #, %, or : retain their original meaning.
func normalizeDSN(raw string) (string, error) {
	dsn := strings.TrimPrefix(raw, "mysql://")
	boundary := strings.LastIndex(dsn, "@tcp(")
	credentials := ""
	start := 0
	if boundary >= 0 {
		credentials = dsn[:boundary]
		start = boundary + 1
	} else if !strings.HasPrefix(dsn, "tcp(") {
		return raw, nil
	}
	remaining := dsn[start+4:]
	end := strings.Index(remaining, ")/")
	if end < 0 {
		return "", fmt.Errorf("invalid MySQL tcp DSN")
	}
	address := remaining[:end]
	database, query, _ := strings.Cut(remaining[end+2:], "?")
	u := url.URL{Scheme: "mysql", Host: address, Path: "/" + database, RawQuery: query}
	if credentials != "" {
		user, password, hasPassword := strings.Cut(credentials, ":")
		if hasPassword {
			u.User = url.UserPassword(user, password)
		} else {
			u.User = url.User(user)
		}
	}
	return u.String(), nil
}

func isDateType(kind byte) bool {
	switch kind {
	case mysql.MYSQL_TYPE_DATE, mysql.MYSQL_TYPE_NEWDATE, mysql.MYSQL_TYPE_DATETIME, mysql.MYSQL_TYPE_TIMESTAMP, mysql.MYSQL_TYPE_DATETIME2, mysql.MYSQL_TYPE_TIMESTAMP2:
		return true
	}
	return false
}
func parseDate(value string, location *time.Location) (time.Time, error) {
	if value == "" || strings.HasPrefix(value, "0000-00-00") {
		return time.Time{}, nil
	}
	if location == nil {
		location = time.UTC
	}
	layout := "2006-01-02 15:04:05.999999999"
	if len(value) == 10 {
		layout = "2006-01-02"
	}
	return time.ParseInLocation(layout, value, location)
}
