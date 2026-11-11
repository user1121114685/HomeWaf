package wafpipeline

import (
	"HomeWaf/internal/bodyinspect"
	"HomeWaf/wafenginecore/uploadscan"
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"

	"golang.org/x/text/unicode/norm"
)

type ViewLimits struct {
	BodyBytes       int64 `json:"body_bytes"`
	ExpansionRatio  int64 `json:"expansion_ratio"`
	MaxFields       int   `json:"max_fields"`
	FilePrefixBytes int64 `json:"file_prefix_bytes"`
}

func DefaultViewLimits() ViewLimits {
	return ViewLimits{BodyBytes: 1 << 20, ExpansionRatio: 32, MaxFields: 2048, FilePrefixBytes: 64 << 10}
}
func (l ViewLimits) normalized() ViewLimits {
	d := DefaultViewLimits()
	if l.BodyBytes <= 0 || l.BodyBytes > 16<<20 {
		l.BodyBytes = d.BodyBytes
	}
	if l.ExpansionRatio <= 0 || l.ExpansionRatio > 100 {
		l.ExpansionRatio = d.ExpansionRatio
	}
	if l.MaxFields <= 0 || l.MaxFields > 10000 {
		l.MaxFields = d.MaxFields
	}
	if l.FilePrefixBytes <= 0 || l.FilePrefixBytes > 1<<20 {
		l.FilePrefixBytes = d.FilePrefixBytes
	}
	return l
}

type Field struct {
	Source   string `json:"source"`
	Name     string `json:"name"`
	Value    string `json:"value"`
	RawValue string `json:"-"`
}
type FilePart struct {
	Name        string
	Filename    string
	Filenames   []string
	ContentType string
	Prefix      []byte
	Truncated   bool
	Size        int64
}
type NormalizedView struct {
	Request          *http.Request
	Path             string
	RawPath          string
	Fields           []Field
	Files            []FilePart
	Body             []byte
	BodyTruncated    bool
	ParseError       string
	limits           ViewLimits
	once             sync.Once
	bodyErr          error
	deadline         time.Time
	scanContext      context.Context
	excludeFields    []string
	bodyReadDuration time.Duration
	bodyReadStarted  bool
}

func NormalizeValue(s string) string {
	for i := 0; i < 3; i++ {
		d, e := url.QueryUnescape(s)
		if e != nil || d == s {
			break
		}
		s = d
	}
	return norm.NFKC.String(html.UnescapeString(strings.ReplaceAll(s, "\x00", "")))
}
func NormalizePath(s string) string {
	for i := 0; i < 3; i++ {
		d, e := url.PathUnescape(s)
		if e != nil || d == s {
			break
		}
		s = d
	}
	s = strings.ReplaceAll(strings.ReplaceAll(s, "\\", "/"), "\x00", "")
	segments := strings.Split(s, "/")
	for i, v := range segments {
		if n := strings.IndexByte(v, ';'); n >= 0 {
			segments[i] = v[:n]
		}
	}
	return path.Clean("/" + strings.TrimLeft(norm.NFKC.String(strings.Join(segments, "/")), "/"))
}
func NewNormalizedView(r *http.Request, l ViewLimits) *NormalizedView {
	v := &NormalizedView{Request: r, RawPath: r.URL.EscapedPath(), Path: NormalizePath(r.URL.EscapedPath()), limits: l.normalized()}
	query := r.URL.Query()
	keys := make([]string, 0, len(query))
	for k := range query {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		values := query[k]
		for _, s := range values {
			v.add("query", k, s)
		}
	}
	for _, cookie := range r.Cookies() {
		if strings.HasPrefix(cookie.Name, "hw_gate_") || strings.HasPrefix(cookie.Name, "hw_csrf_") || strings.HasPrefix(cookie.Name, "hw_challenge_") {
			continue
		}
		v.add("cookie", cookie.Name, cookie.Value)
	}
	// Inspect common and custom textual headers, excluding authentication data.
	keys = keys[:0]
	for k := range r.Header {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if strings.EqualFold(k, "Cookie") || strings.EqualFold(k, "Authorization") || strings.EqualFold(k, "Proxy-Authorization") || strings.EqualFold(k, "X-HomeWaf-Token") {
			continue
		}
		for _, value := range r.Header.Values(k) {
			v.add("header", k, value)
		}
	}
	return v
}
func (v *NormalizedView) add(source, name, value string) {
	if len(v.Fields) >= v.limits.MaxFields {
		v.BodyTruncated = true
		return
	}
	if len(name) > 8192 {
		name = name[:8192]
	}
	if int64(len(value)) > v.limits.BodyBytes {
		value = value[:v.limits.BodyBytes]
		v.BodyTruncated = true
	}
	v.Fields = append(v.Fields, Field{Source: source, Name: NormalizeValue(name), Value: NormalizeValue(value), RawValue: value})
}

// ReadBody retains the original wire representation, including compression and
// bytes beyond the inspection prefix. Close still reaches the original body.
func (v *NormalizedView) ReadBody() error { v.once.Do(v.readBody); return v.bodyErr }
func (v *NormalizedView) readBody() {
	v.bodyReadStarted = true
	r := v.Request
	if r.Body == nil || r.Body == http.NoBody {
		return
	}
	readStart := time.Now()
	wire, err := bodyinspect.ReadPrefix(r, v.limits.BodyBytes)
	v.bodyReadDuration = time.Since(readStart)
	if err != nil {
		v.bodyErr = fmt.Errorf("body read failed")
		v.ParseError = "body_read_error"
		return
	}
	v.BodyTruncated = v.BodyTruncated || int64(len(wire)) > v.limits.BodyBytes
	inspected := wire
	if int64(len(wire)) > v.limits.BodyBytes {
		inspected = wire[:v.limits.BodyBytes]
	}
	encoding := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Encoding")))
	switch encoding {
	case "", "identity":
		v.Body = append([]byte(nil), inspected...)
	case "gzip", "deflate", "br", "zstd":
		var decoder io.ReadCloser
		if encoding == "gzip" {
			decoder, err = gzip.NewReader(bytes.NewReader(inspected))
		} else if encoding == "deflate" {
			decoder, err = zlib.NewReader(bytes.NewReader(inspected))
			if err != nil {
				decoder = flate.NewReader(bytes.NewReader(inspected))
				err = nil
			}
		} else if encoding == "br" {
			decoder = io.NopCloser(brotli.NewReader(bytes.NewReader(inspected)))
		} else {
			var z *zstd.Decoder
			z, err = zstd.NewReader(bytes.NewReader(inspected), zstd.WithDecoderConcurrency(1), zstd.WithDecoderLowmem(true), zstd.WithDecoderMaxMemory(32<<20), zstd.WithDecoderMaxWindow(8<<20))
			if err == nil {
				decoder = z.IOReadCloser()
			}
		}
		if err != nil {
			v.ParseError = "invalid_content_encoding"
			return
		}
		defer decoder.Close()
		limit := v.limits.BodyBytes
		if expansion := int64(len(inspected)) * v.limits.ExpansionRatio; expansion < limit {
			limit = expansion
		}
		data, readErr := io.ReadAll(io.LimitReader(decoder, limit+1))
		if int64(len(data)) > limit {
			v.BodyTruncated = true
			data = data[:limit]
		}
		if readErr != nil {
			v.ParseError = "incomplete_content_encoding"
		}
		v.Body = data
	default:
		v.ParseError = "unsupported_content_encoding"
		return
	}
	ct, params, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	switch {
	case ct == "application/x-www-form-urlencoded":
		values, err := url.ParseQuery(string(v.Body))
		if err != nil {
			v.ParseError = "invalid_form"
		}
		for k, vs := range values {
			for _, s := range vs {
				v.add("form", k, s)
			}
		}
	case ct == "application/json" || strings.HasSuffix(ct, "+json"):
		v.readJSON()
	case strings.HasPrefix(ct, "multipart/"):
		if params["boundary"] == "" {
			v.ParseError = "missing_multipart_boundary"
			return
		}
		v.readMultipart(params["boundary"])
	default:
		if len(v.Body) > 0 {
			v.add("body", "", string(v.Body))
		}
	}
}

type onceError struct{ err error }

func (e *onceError) Read([]byte) (int, error) {
	if e.err == nil {
		return 0, io.EOF
	}
	err := e.err
	e.err = nil
	return 0, err
}

// Token iteration inspects duplicate object keys as well as values. Decoding
// straight into a map would silently discard all but the last duplicate value.
func (v *NormalizedView) readJSON() {
	type frame struct {
		object, keyNext bool
		name, key       string
	}
	stack := []frame{}
	decoder := json.NewDecoder(bytes.NewReader(v.Body))
	decoder.UseNumber()
	tokens := 0
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			if len(stack) > 0 {
				v.ParseError = "incomplete_json"
			}
			return
		}
		if err != nil {
			v.ParseError = "invalid_json"
			return
		}
		tokens++
		if tokens > v.limits.MaxFields*4 || len(stack) > 32 || len(v.Fields) >= v.limits.MaxFields {
			v.BodyTruncated = true
			return
		}
		name := ""
		if len(stack) > 0 {
			f := &stack[len(stack)-1]
			name = f.name
			if f.object {
				name = f.key
			}
		}
		if delim, ok := token.(json.Delim); ok {
			if delim == '{' || delim == '[' {
				if len(stack) > 0 && stack[len(stack)-1].object {
					stack[len(stack)-1].keyNext = true
				}
				stack = append(stack, frame{object: delim == '{', keyNext: delim == '{', name: name})
				continue
			}
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			continue
		}
		if len(stack) > 0 {
			f := &stack[len(stack)-1]
			if f.object && f.keyNext {
				key, ok := token.(string)
				if !ok {
					v.ParseError = "invalid_json_key"
					return
				}
				f.key = key
				f.keyNext = false
				v.add("json_key", key, key)
				continue
			}
			if f.object {
				f.keyNext = true
			}
		}
		if token != nil {
			v.add("json", name, fmt.Sprint(token))
		}
	}
}
func (v *NormalizedView) readMultipart(boundary string) {
	reader := multipart.NewReader(bytes.NewReader(v.Body), boundary)
	for n := 0; n < v.limits.MaxFields; n++ {
		part, err := reader.NextRawPart()
		if err == io.EOF {
			return
		}
		if err != nil {
			v.ParseError = "incomplete_multipart"
			return
		}
		names := uploadscan.PartFileNames(part)
		if len(names) == 0 {
			data, _ := io.ReadAll(io.LimitReader(part, v.limits.BodyBytes+1))
			v.add("multipart", part.FormName(), string(data))
		} else {
			data, _ := io.ReadAll(io.LimitReader(part, v.limits.FilePrefixBytes+1))
			size := int64(len(data))
			rest, _ := io.Copy(io.Discard, part)
			size += rest
			truncated := int64(len(data)) > v.limits.FilePrefixBytes
			if truncated {
				data = data[:v.limits.FilePrefixBytes]
			}
			for i := range names {
				names[i] = NormalizeValue(names[i])
			}
			v.Files = append(v.Files, FilePart{Name: NormalizeValue(part.FormName()), Filename: names[0], Filenames: names, ContentType: part.Header.Get("Content-Type"), Prefix: data, Truncated: truncated, Size: size})
		}
		_ = part.Close()
	}
	v.BodyTruncated = true
}
