package waf_service

import (
	"HomeWaf/model"
	"HomeWaf/utils"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net"
	"net/http"
	"net/netip"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type dashboardNodeCache struct {
	sync.Mutex
	data       model.DashboardTarget
	next, last time.Time
	running    bool
	lookup     func(context.Context) (model.DashboardTarget, error)
}

var dashboardNodeLocation = &dashboardNodeCache{lookup: discoverDashboardNode}

// Nonblocking, single-flight discovery: a slow provider cannot stall statistics.
func (c *dashboardNodeCache) snapshot() model.DashboardTarget {
	c.Lock()
	defer c.Unlock()
	if !c.running && !time.Now().Before(c.next) {
		c.running, c.last = true, time.Now()
		c.data = model.DashboardTarget{Status: "locating", Message: "正在获取服务器公网 IP 与城市"}
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
			defer cancel()
			data, err := c.lookup(ctx)
			ttl := 30 * time.Minute
			if err != nil {
				ttl = 5 * time.Minute
				data = model.DashboardTarget{Status: "unavailable", Message: "公网 IP 定位失败，5 分钟后自动重试；也可保存节点设置重试"}
			}
			data.CheckedAt = time.Now().UnixMilli()
			c.Lock()
			c.data, c.next, c.running = data, time.Now().Add(ttl), false
			c.Unlock()
			InvalidateScreenSnapshot()
		}()
	}
	return c.data
}

// Administrative saves may retry, but cannot flood free discovery providers.
func RefreshDashboardNodeLocation() {
	dashboardNodeLocation.Lock()
	defer dashboardNodeLocation.Unlock()
	if !dashboardNodeLocation.running && time.Since(dashboardNodeLocation.last) >= time.Minute {
		dashboardNodeLocation.next = time.Time{}
	}
}

var dashboardNodeClient = &http.Client{
	Timeout: 5 * time.Second,
	Transport: utils.SafeOutboundDirectTransport(func(address string) error {
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return err
		}
		if ok, reason := utils.IsSafeOutboundHost(host); !ok {
			return errors.New(reason)
		}
		return nil
	}),
	CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
}

func discoverDashboardNode(ctx context.Context) (model.DashboardTarget, error) {
	return discoverDashboardNodeWithClient(ctx, dashboardNodeClient)
}

func discoverDashboardNodeWithClient(ctx context.Context, client *http.Client) (model.DashboardTarget, error) {
	// Prefer IPIP (mainland China). IPinfo is a fallback, not a browser dependency.
	for _, provider := range []struct{ url, name string }{
		{"https://myip.ipip.net/json", "ipip.net"},
		{"https://ipinfo.io/json", "ipinfo.io"},
	} {
		data, err := fetchDashboardNode(ctx, client, provider.url, provider.name)
		if err == nil {
			return data, nil
		}
	}
	return model.DashboardTarget{}, errors.New("公网 IP 定位服务暂不可用")
}

func fetchDashboardNode(ctx context.Context, client *http.Client, endpoint, provider string) (model.DashboardTarget, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return model.DashboardTarget{}, err
	}
	req.Header.Set("Accept", "application/json, text/plain")
	req.Header.Set("User-Agent", "HomeWaf-NodeLocation/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return model.DashboardTarget{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return model.DashboardTarget{}, errors.New("定位服务未返回成功状态")
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 16385))
	if err != nil || len(body) > 16384 {
		return model.DashboardTarget{}, errors.New("定位服务响应无效")
	}
	return decodeDashboardNode(body, provider)
}

var ipipText = regexp.MustCompile(`当前\s*IP[：:]\s*(\S+)\s+来自于[：:]\s*(.+)`)

func decodeDashboardNode(body []byte, provider string) (model.DashboardTarget, error) {
	out := model.DashboardTarget{Mode: "auto", Source: provider, Status: "ready"}
	if provider == "ipip.net" {
		var payload struct {
			Ret  string `json:"ret"`
			Data struct {
				IP       string   `json:"ip"`
				Location []string `json:"location"`
			} `json:"data"`
		}
		if json.Unmarshal(body, &payload) == nil && payload.Ret == "ok" {
			out.PublicIP = payload.Data.IP
			if len(payload.Data.Location) > 0 {
				out.Country = payload.Data.Location[0]
			}
			if len(payload.Data.Location) > 1 {
				out.Region = payload.Data.Location[1]
			}
			if len(payload.Data.Location) > 2 {
				out.City = payload.Data.Location[2]
			}
		} else if match := ipipText.FindStringSubmatch(strings.TrimSpace(string(body))); len(match) == 3 {
			out.PublicIP = match[1]
			parts := strings.Fields(match[2])
			if len(parts) >= 3 {
				out.Country, out.Region, out.City = parts[0], parts[1], parts[2]
			}
		}
	} else {
		var payload struct {
			IP      string `json:"ip"`
			City    string `json:"city"`
			Region  string `json:"region"`
			Country string `json:"country"`
			Loc     string `json:"loc"`
			Bogon   bool   `json:"bogon"`
		}
		if err := json.Unmarshal(body, &payload); err != nil || payload.Bogon {
			return out, errors.New("IPinfo 返回无效数据")
		}
		out.PublicIP, out.City, out.Region, out.Country = payload.IP, payload.City, payload.Region, payload.Country
		if parts := strings.Split(payload.Loc, ","); len(parts) == 2 {
			lat, latErr := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
			lon, lonErr := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
			if latErr == nil && lonErr == nil && !math.IsNaN(lat) && !math.IsNaN(lon) && math.Abs(lat) <= 90 && math.Abs(lon) <= 180 {
				out.Latitude, out.Longitude, out.HasCoordinates = lat, lon, true
			}
		}
	}
	ip := net.ParseIP(strings.TrimSpace(out.PublicIP))
	if ip == nil || !ip.IsGlobalUnicast() {
		return out, errors.New("公网 IP 无效")
	}
	if ok, _ := utils.IsSafeOutboundHost(ip.String()); !ok {
		return out, errors.New("返回的 IP 不是公网地址")
	}
	address, _ := netip.ParseAddr(ip.String())
	for _, reserved := range []string{"192.0.2.0/24", "198.51.100.0/24", "203.0.113.0/24", "2001:db8::/32"} {
		if netip.MustParsePrefix(reserved).Contains(address) {
			return out, errors.New("返回了文档示例地址")
		}
	}
	out.PublicIP = ip.String()
	out.Country, out.Region, out.City = strings.TrimSpace(out.Country), strings.TrimSpace(out.Region), strings.TrimSpace(out.City)
	if out.Country == "" || (out.City == "" && !out.HasCoordinates) || len(out.Country) > 128 || len(out.Region) > 128 || len(out.City) > 128 {
		return out, errors.New("公网 IP 归属城市缺失")
	}
	return out, nil
}
