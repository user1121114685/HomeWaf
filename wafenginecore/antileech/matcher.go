// Package antileech shares the existing referer policy with port snapshots.
package antileech

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

type Matcher struct {
	files       *regexp.Regexp
	entries     []string
	expressions map[string]*regexp.Regexp
}

func Compile(fileTypes, referers string) (*Matcher, error) {
	m := &Matcher{entries: strings.Split(referers, ";"), expressions: map[string]*regexp.Regexp{}}
	if fileTypes != "" {
		var err error
		m.files, err = regexp.Compile(`(?i).*\.(` + fileTypes + `)$`)
		if err != nil {
			return nil, fmt.Errorf("invalid protected file expression")
		}
	}
	for i, entry := range m.entries {
		entry = strings.TrimSpace(entry)
		m.entries[i] = entry
		if strings.HasPrefix(entry, "~") {
			re, err := regexp.Compile(entry[1:])
			if err != nil {
				return nil, fmt.Errorf("invalid referer expression")
			}
			m.expressions[entry] = re
		}
	}
	return m, nil
}

func (m *Matcher) Protected(path string) bool {
	return m != nil && m.files != nil && m.files.MatchString(path)
}
func (m *Matcher) Allowed(referer string, hosts []string) bool {
	if m == nil {
		return true
	}
	host := ""
	parsed, err := url.Parse(referer)
	valid := err == nil && parsed != nil && parsed.Hostname() != "" && parsed.User == nil && (parsed.Scheme == "http" || parsed.Scheme == "https")
	if valid {
		host = strings.ToLower(parsed.Hostname())
	}
	for _, entry := range m.entries {
		switch entry {
		case "none":
			if referer == "" {
				return true
			}
		case "blocked":
			if referer != "" && !valid {
				return true
			}
		case "server_names":
			if valid {
				for _, name := range hosts {
					if strings.EqualFold(strings.Trim(name, "[]"), host) {
						return true
					}
				}
			}
		default:
			if !valid || entry == "" {
				continue
			}
			if re := m.expressions[entry]; re != nil {
				if re.MatchString(host) {
					return true
				}
				continue
			}
			entry = strings.ToLower(entry)
			if strings.HasPrefix(entry, "*.") {
				if strings.HasSuffix(host, entry[1:]) {
					return true
				}
			} else if host == entry || strings.EqualFold(parsed.Host, entry) {
				return true
			}
		}
	}
	return false
}
