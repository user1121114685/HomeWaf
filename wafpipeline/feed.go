package wafpipeline

import (
	"HomeWaf/wafenginecore/ipset"
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

const FeedFeature = "threat_feed"
const MaxFeedEntries = 500000

type FeedEntry struct {
	CIDR      string `json:"cidr"`
	Reason    string `json:"reason"`
	Score     int    `json:"score"`
	ExpiresAt int64  `json:"expires_at,omitempty"`
}
type FeedPayload struct {
	Version  string      `json:"version"`
	Sequence uint64      `json:"sequence"`
	Add      []FeedEntry `json:"add"`
	Remove   []string    `json:"remove"`
}
type SignedDelta struct {
	FeedPayload
	KeyID     string `json:"key_id"`
	Signature string `json:"signature"`
}

func FeedSigningMessage(payload FeedPayload, keyID string) []byte {
	data, _ := json.Marshal(payload)
	return append([]byte("HomeWaf threat feed v1\n"+keyID+"\n"), data...)
}

type FeedProvider interface {
	Name() string
	Fetch(context.Context, string, string) (*SignedDelta, string, error)
}

var ErrFeedNotModified = errors.New("feed not modified")

type HTTPSFeedProvider struct {
	URL    string
	Token  string
	Client *http.Client
}

func (p *HTTPSFeedProvider) Name() string { return "signed-https" }
func (p *HTTPSFeedProvider) Fetch(ctx context.Context, since, etag string) (*SignedDelta, string, error) {
	u, err := url.Parse(p.URL)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
		return nil, "", errors.New("feed requires configured HTTPS URL without credentials")
	}
	q := u.Query()
	q.Set("since", since)
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, "", errors.New("invalid feed request")
	}
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	if p.Token != "" {
		req.Header.Set("Authorization", "Bearer "+p.Token)
	}
	client := p.Client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	copy := *client
	copy.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	res, err := copy.Do(req)
	if err != nil {
		return nil, "", errors.New("feed connection failed")
	}
	defer res.Body.Close()
	if res.StatusCode == 304 {
		return nil, etag, ErrFeedNotModified
	}
	if res.StatusCode != 200 {
		return nil, "", fmt.Errorf("feed returned HTTP %d", res.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, 64<<20+1))
	if err != nil || len(data) > 64<<20 {
		return nil, "", errors.New("feed exceeds download budget or is incomplete")
	}
	var delta SignedDelta
	d := json.NewDecoder(bytesReader(data))
	d.DisallowUnknownFields()
	if err = d.Decode(&delta); err != nil {
		return nil, "", errors.New("invalid signed feed document")
	}
	if d.Decode(new(any)) != io.EOF {
		return nil, "", errors.New("feed contains trailing data")
	}
	return &delta, res.Header.Get("ETag"), nil
}

// Small local wrapper keeps the provider independent of any application clients.
func bytesReader(data []byte) io.Reader { return bytes.NewReader(data) }

type FeedCache struct {
	Version  string      `json:"version"`
	Sequence uint64      `json:"sequence"`
	ETag     string      `json:"etag"`
	Entries  []FeedEntry `json:"entries"`
}
type feedSnapshot struct {
	cache   FeedCache
	matcher *ipset.TimedSet
}
type FeedStatus struct {
	Enabled     bool   `json:"enabled"`
	Version     string `json:"version"`
	Sequence    uint64 `json:"sequence"`
	Entries     int    `json:"entries"`
	LastSuccess int64  `json:"last_success"`
	LastError   string `json:"last_error,omitempty"`
	Dropped     int    `json:"dropped"`
	NextAttempt int64  `json:"next_attempt"`
}
type SignedFeed struct {
	mu       sync.Mutex
	snapshot atomic.Pointer[feedSnapshot]
	enabled  atomic.Bool
	status   FeedStatus
	provider FeedProvider
	keys     map[string]ed25519.PublicKey
	licensed func() bool
	persist  func(FeedCache) error
	interval time.Duration
	failures int
}

func NewSignedFeed(licensed func() bool, persist func(FeedCache) error) *SignedFeed {
	return &SignedFeed{licensed: licensed, persist: persist, interval: time.Hour}
}
func (f *SignedFeed) Configure(enabled bool, p FeedProvider, keys map[string]ed25519.PublicKey, interval time.Duration) error {
	return f.ConfigureTransaction(enabled, p, keys, interval, false, nil)
}

// ConfigureTransaction serializes source persistence with a running refresh.
// The old matcher stays available until a valid configuration is committed.
func (f *SignedFeed) ConfigureTransaction(enabled bool, p FeedProvider, keys map[string]ed25519.PublicKey, interval time.Duration, reset bool, commit func() error) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if enabled && (p == nil || len(keys) == 0 || f.licensed == nil || !f.licensed()) {
		return errors.New("签名订阅需要有效threat_feed授权和非空可信公钥库")
	}
	if interval < time.Minute || interval > 24*time.Hour {
		return errors.New("订阅间隔须为1分钟至24小时")
	}
	if commit != nil {
		if err := commit(); err != nil {
			return err
		}
	}
	if reset {
		f.snapshot.Store(nil)
		f.status = FeedStatus{}
		f.failures = 0
	}
	f.provider = p
	f.keys = make(map[string]ed25519.PublicKey, len(keys))
	for id, key := range keys {
		f.keys[id] = append([]byte(nil), key...)
	}
	f.interval = interval
	f.enabled.Store(enabled)
	f.status.Enabled = enabled
	f.status.NextAttempt = 0
	return nil
}
func (f *SignedFeed) Restore(cache FeedCache) error {
	if len(cache.Entries) > MaxFeedEntries {
		return errors.New("cached feed exceeds quota")
	}
	snap, err := buildFeedSnapshot(cache)
	if err != nil {
		return err
	}
	f.snapshot.Store(snap)
	return nil
}
func buildFeedSnapshot(cache FeedCache) (*feedSnapshot, error) {
	entries := make([]ipset.TimedEntry, 0, len(cache.Entries))
	for _, e := range cache.Entries {
		prefix, err := netip.ParsePrefix(e.CIDR)
		if err != nil || e.ExpiresAt < 0 || len(e.Reason) > 256 || e.Score < 0 || e.Score > 100 {
			return nil, errors.New("invalid feed entry")
		}
		entries = append(entries, ipset.TimedEntry{Value: prefix.Masked().String(), ExpiresAt: e.ExpiresAt})
	}
	m, err := ipset.BuildTimedSet(entries)
	if err != nil {
		return nil, err
	}
	return &feedSnapshot{cache: cache, matcher: m}, nil
}
func (f *SignedFeed) Contains(ip netip.Addr, now int64) bool {
	if protectedAddress(ip) {
		return false
	}
	return f.ContainsForDiagnostics(ip, now)
}

// This lookup never authorizes a denial. It exposes protected overlaps solely
// to the IP stage's ignored-match diagnostics; Contains keeps its safe default.
func (f *SignedFeed) ContainsForDiagnostics(ip netip.Addr, now int64) bool {
	if !f.enabled.Load() || !ip.IsValid() {
		return false
	}
	snap := f.snapshot.Load()
	return snap != nil && snap.matcher.Contains(ip.AsSlice(), now)
}
func (f *SignedFeed) Status() FeedStatus {
	f.mu.Lock()
	defer f.mu.Unlock()
	s := f.status
	if snap := f.snapshot.Load(); snap != nil {
		s.Version = snap.cache.Version
		s.Sequence = snap.cache.Sequence
		s.Entries = len(snap.cache.Entries)
	}
	return s
}
func (f *SignedFeed) Sync(ctx context.Context, force bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.enabled.Load() {
		return nil
	}
	if f.licensed == nil || !f.licensed() {
		f.enabled.Store(false)
		f.status.Enabled = false
		f.status.LastError = "订阅授权失效"
		return errors.New(f.status.LastError)
	}
	now := time.Now()
	if !force && f.status.NextAttempt > now.Unix() {
		return nil
	}
	old := FeedCache{}
	if snap := f.snapshot.Load(); snap != nil {
		old = snap.cache
	}
	deadline, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	delta, etag, err := f.provider.Fetch(deadline, old.Version, old.ETag)
	if errors.Is(err, ErrFeedNotModified) {
		f.status.LastSuccess = now.Unix()
		f.status.NextAttempt = now.Add(f.interval).Unix()
		f.failures = 0
		f.status.LastError = ""
		return nil
	}
	if err == nil {
		err = f.apply(delta, etag, old)
	}
	if err != nil {
		f.failures++
		power := f.failures
		if power > 6 {
			power = 6
		}
		retry := time.Duration(1<<power) * time.Minute
		if retry > time.Hour {
			retry = time.Hour
		}
		f.status.NextAttempt = now.Add(retry).Unix()
		f.status.LastError = err.Error()
		return err
	}
	f.status.LastError = ""
	f.status.LastSuccess = now.Unix()
	f.status.NextAttempt = now.Add(f.interval).Unix()
	f.failures = 0
	return nil
}
func (f *SignedFeed) apply(delta *SignedDelta, etag string, old FeedCache) error {
	if delta == nil || delta.Version == "" || len(delta.Version) > 128 || len(delta.KeyID) > 128 || delta.Sequence <= old.Sequence {
		return errors.New("feed version must advance monotonically")
	}
	key := f.keys[delta.KeyID]
	sig, err := base64.RawURLEncoding.DecodeString(delta.Signature)
	if err != nil || len(key) != ed25519.PublicKeySize || !ed25519.Verify(key, FeedSigningMessage(delta.FeedPayload, delta.KeyID), sig) {
		return errors.New("untrusted feed signature")
	}
	if len(delta.Add) > MaxFeedEntries*2 || len(delta.Remove) > MaxFeedEntries {
		return errors.New("feed delta exceeds entry quota")
	}
	merged := make(map[string]FeedEntry, len(old.Entries)+len(delta.Add))
	for _, e := range old.Entries {
		merged[e.CIDR] = e
	}
	for _, raw := range delta.Remove {
		prefix, e := netip.ParsePrefix(raw)
		if e != nil {
			return errors.New("invalid removed prefix")
		}
		delete(merged, prefix.Masked().String())
	}
	for _, e := range delta.Add {
		prefix, err := netip.ParsePrefix(e.CIDR)
		if err != nil || e.ExpiresAt < 0 || len(e.Reason) > 256 || e.Score < 0 || e.Score > 100 {
			return errors.New("invalid feed entry")
		}
		e.CIDR = prefix.Masked().String()
		merged[e.CIDR] = e
	}
	entries := make([]FeedEntry, 0, len(merged))
	for _, e := range merged {
		if e.ExpiresAt == 0 || e.ExpiresAt > time.Now().Unix() {
			entries = append(entries, e)
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Score == entries[j].Score {
			return entries[i].CIDR < entries[j].CIDR
		}
		return entries[i].Score > entries[j].Score
	})
	f.status.Dropped = 0
	if len(entries) > MaxFeedEntries {
		f.status.Dropped = len(entries) - MaxFeedEntries
		entries = entries[:MaxFeedEntries]
	}
	cache := FeedCache{Version: delta.Version, Sequence: delta.Sequence, ETag: etag, Entries: entries}
	snap, err := buildFeedSnapshot(cache)
	if err != nil {
		return err
	}
	if f.persist != nil {
		if err = f.persist(cache); err != nil {
			return errors.New("could not persist authenticated feed cache")
		}
	}
	f.snapshot.Store(snap)
	return nil
}
func (f *SignedFeed) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	_ = f.Sync(ctx, false)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = f.Sync(ctx, false)
		}
	}
}
func (f *SignedFeed) Reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.snapshot.Store(nil)
	f.status.Version = ""
	f.status.Sequence = 0
	f.status.Entries = 0
	f.status.LastError = ""
	f.status.NextAttempt = 0
	f.failures = 0
}
func FeedVersion(sequence uint64) string { return strconv.FormatUint(sequence, 10) }
