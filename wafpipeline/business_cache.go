package wafpipeline

import (
	"HomeWaf/cache"
	"HomeWaf/model"
	"HomeWaf/wafenginecore/wafwebcache"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type BusinessCacheConfig struct {
	Rules          []model.CacheRule `json:"rules"`
	MaxObjectBytes int               `json:"max_object_bytes,omitempty"`
}
type preparedBusinessCache struct {
	store     *cache.WafCache
	rules     []model.CacheRule
	namespace string
	maxObject int
	metrics   *PortMetrics
}
type cachedBusinessResponse struct {
	Header   http.Header
	Body     []byte
	StoredAt int64
}

func validateBusinessCache(cfg *BusinessCacheConfig) error {
	if cfg == nil {
		return nil
	}
	if len(cfg.Rules) > 256 || cfg.MaxObjectBytes < 0 || cfg.MaxObjectBytes > 65536 {
		return errors.New("invalid business cache quota")
	}
	for _, rule := range cfg.Rules {
		if rule.RuleType < 1 || rule.RuleType > 3 || len(rule.RuleContent) > 2048 || len(rule.RequestMethod) > 100 || rule.CacheTime < 0 || rule.CacheTime > 86400 || (rule.ParamType != 1 && rule.ParamType != 2) {
			return errors.New("invalid business cache rule")
		}
	}
	return nil
}
func prepareBusinessCache(s *Snapshot, target *ProxyTarget, route UpstreamRoute) {
	if route.Cache == nil || len(route.Cache.Rules) == 0 || route.HeaderProfile != "" || target.handler != nil {
		return
	}
	quota := (64 << 20) / max(1, Ports.Limit())
	maxObject := min(65536, quota/3)
	if route.Cache.MaxObjectBytes > 0 {
		maxObject = min(maxObject, route.Cache.MaxObjectBytes)
	}
	prepared := &preparedBusinessCache{store: s.Compiled.Metrics.cacheStore(), rules: wafwebcache.PrepareCacheRules(route.Cache.Rules), namespace: randomToken(), maxObject: maxObject, metrics: s.Compiled.Metrics}
	target.cache = prepared
	if target.balance != nil {
		for _, backend := range target.balance.targets {
			backend.cache = prepared
		}
	}
}
func (c *preparedBusinessCache) requestKey(r *http.Request) (string, int) {
	if r.Method != "GET" && r.Method != "HEAD" || r.ContentLength != 0 || r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" || r.Header.Get("Range") != "" || r.Header.Get("Upgrade") != "" || strings.Contains(strings.ToLower(r.Header.Get("Cache-Control")), "no-cache") {
		return "", 0
	}
	for name := range r.Header {
		if strings.HasPrefix(strings.ToLower(name), "if-") {
			return "", 0
		}
	}
	ttl, err := wafwebcache.MatchSortedCacheRules(c.rules, r)
	if err != nil || ttl <= 0 {
		return "", 0
	}
	headers, _ := json.Marshal(r.Header)
	sum := sha256.Sum256([]byte(r.Host + "\n" + r.URL.RequestURI() + "\n" + string(headers)))
	return c.namespace + ":" + hex.EncodeToString(sum[:]), ttl
}
func (c *preparedBusinessCache) serve(w http.ResponseWriter, r *http.Request, record *forwardResponse) bool {
	if record.login != nil {
		return false
	}
	key, _ := c.requestKey(r)
	if key == "" {
		return false
	}
	raw, err := c.store.GetBytes(key)
	if err != nil || len(raw) == 0 {
		c.metrics.CacheMisses.Add(1)
		return false
	}
	var response cachedBusinessResponse
	if json.Unmarshal(raw, &response) != nil {
		c.store.Remove(key)
		return false
	}
	for name, values := range response.Header {
		w.Header()[name] = append([]string{}, values...)
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(response.Body)))
	w.Header().Set("Age", strconv.FormatInt(max(0, time.Now().Unix()-response.StoredAt), 10))
	record.status = 200
	record.cacheHit = true
	record.location = response.Header.Get("Location")
	record.contentType = response.Header.Get("Content-Type")
	w.WriteHeader(200)
	if r.Method != "HEAD" {
		_, _ = w.Write(response.Body)
	}
	c.metrics.CacheHits.Add(1)
	return true
}
func (c *preparedBusinessCache) capture(response *http.Response, state *forwardRequest) {
	if bodyResponsePolicyActive(state.snapshot, state.request) {
		return
	}
	if state.request.Request.Method != "GET" || state.response.login != nil || response.StatusCode != 200 || response.Header.Get("Set-Cookie") != "" || response.Header.Get("Vary") != "" || response.ContentLength > int64(c.maxObject) || strings.Contains(strings.ToLower(response.Header.Get("Content-Type")), "stream") {
		return
	}
	control := strings.ToLower(response.Header.Get("Cache-Control"))
	if strings.Contains(control, "private") || strings.Contains(control, "no-store") || strings.Contains(control, "no-cache") {
		return
	}
	key, ttl := c.requestKey(state.request.Request)
	if key == "" {
		return
	}
	header := response.Header.Clone()
	body := &cacheCaptureBody{ReadCloser: response.Body, limit: c.maxObject}
	body.complete = func(data []byte) {
		raw, err := json.Marshal(cachedBusinessResponse{Header: header, Body: data, StoredAt: time.Now().Unix()})
		if err == nil {
			c.store.SetWithTTlRenewTime(key, raw, time.Duration(ttl)*time.Second)
		}
	}
	response.Body = body
}

type cacheCaptureBody struct {
	io.ReadCloser
	data                []byte
	limit               int
	discarded, finished bool
	complete            func([]byte)
}

func (c *cacheCaptureBody) Read(p []byte) (int, error) {
	n, err := c.ReadCloser.Read(p)
	if !c.discarded {
		if len(c.data)+n > c.limit {
			c.discarded = true
			c.data = nil
		} else {
			c.data = append(c.data, p[:n]...)
		}
	}
	if err == io.EOF && !c.finished {
		c.finished = true
		if !c.discarded {
			c.complete(c.data)
		}
		c.data = nil
	}
	return n, err
}
