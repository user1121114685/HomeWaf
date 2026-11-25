package wafpipeline

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"golang.org/x/net/http/httpguts"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
)

// SingleRequest describes one parsed HTTP request, not packets to transmit.
type SingleRequest struct {
	SourceIP     string          `json:"source_ip"`
	SourcePort   uint16          `json:"source_port"`
	Method       string          `json:"method"`
	Scheme       string          `json:"scheme"`
	Version      string          `json:"version"`
	Host         string          `json:"host"`
	Path         string          `json:"path"`
	Query        string          `json:"query"`
	Headers      []RequestHeader `json:"headers"`
	Body         string          `json:"body"`
	BodyEncoding string          `json:"body_encoding"`
}
type RequestHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func (in SingleRequest) build(ctx context.Context, config PortConfig) (*http.Request, error) {
	peer, err := netip.ParseAddr(in.SourceIP)
	if err != nil || peer.Zone() != "" || peer.Unmap().IsUnspecified() || peer.IsMulticast() || in.SourcePort == 0 {
		return nil, errors.New("填写有效的 IPv4 或 IPv6 来源地址，来源端口须为 1–65535")
	}
	if len(in.Method) > 64 || !httpguts.ValidHeaderFieldName(in.Method) {
		return nil, errors.New("请求方法须为有效的 HTTP 方法名")
	}
	if len(in.Path)+len(in.Query) > 8192 || !strings.HasPrefix(in.Path, "/") || strings.HasPrefix(in.Path, "//") || strings.ContainsAny(in.Path, "?#") || strings.ContainsAny(in.Query, "#\r\n") {
		return nil, errors.New("路径须以 / 开头；查询串请单独填写，两者合计不超过 8 KiB")
	}
	uri := in.Path
	if in.Query != "" {
		uri += "?" + in.Query
	}
	u, err := url.ParseRequestURI(uri)
	if err != nil || u.IsAbs() || u.Host != "" {
		return nil, errors.New("请求路径或查询串无效")
	}
	body := []byte(in.Body)
	switch in.BodyEncoding {
	case "", "text":
	case "base64":
		if len(in.Body) > base64.StdEncoding.EncodedLen(65536) {
			return nil, errors.New("解码后的正文最多 64 KiB")
		}
		body, err = base64.StdEncoding.Strict().DecodeString(in.Body)
		if err != nil {
			return nil, errors.New("正文不是有效的 Base64")
		}
	default:
		return nil, errors.New("不支持的正文编码")
	}
	if len(body) > 65536 {
		return nil, errors.New("请求正文最多 64 KiB")
	}
	target, err := url.Parse(AssessmentTarget(config))
	if err != nil {
		return nil, errors.New("服务入口无效")
	}
	scheme := in.Scheme
	if scheme == "" {
		scheme = config.Protocol
	}
	if scheme != "https" && scheme != "http" {
		return nil, errors.New("请求协议须为 HTTP 或 HTTPS")
	}
	req, err := http.NewRequestWithContext(ctx, in.Method, scheme+"://"+target.Host+uri, bytes.NewReader(body))
	if err != nil {
		return nil, errors.New("请求格式无效")
	}
	req.RequestURI = uri
	req.RemoteAddr = net.JoinHostPort(peer.Unmap().String(), strconv.Itoa(int(in.SourcePort)))
	if scheme == "https" {
		req.TLS = &tls.ConnectionState{}
	}
	if in.Host != "" {
		if len(in.Host) > 255 || !httpguts.ValidHostHeader(in.Host) || strings.ContainsAny(in.Host, "/@?#\\") {
			return nil, errors.New("Host 须为主机名或 IP，可带端口，不含协议或路径")
		}
		req.Host = in.Host // Virtual HTTP field only. Never changes the service being evaluated.
	}
	switch in.Version {
	case "HTTP/1.0":
		req.Proto, req.ProtoMajor, req.ProtoMinor = "HTTP/1.0", 1, 0
	case "HTTP/1.1":
	case "HTTP/2.0":
		req.Proto, req.ProtoMajor, req.ProtoMinor = "HTTP/2.0", 2, 0
	case "HTTP/3.0":
		req.Proto, req.ProtoMajor, req.ProtoMinor = "HTTP/3.0", 3, 0
	default:
		return nil, errors.New("选择有效的 HTTP 版本")
	}
	if len(in.Headers) > 64 {
		return nil, errors.New("最多可填写 64 行请求头")
	}
	size := 0
	for _, h := range in.Headers {
		size += len(h.Name) + len(h.Value)
		if size > 32768 || !httpguts.ValidHeaderFieldName(h.Name) || !httpguts.ValidHeaderFieldValue(h.Value) {
			return nil, errors.New("请求头名称或值无效，或合计超过 32 KiB")
		}
		switch strings.ToLower(h.Name) {
		case "host":
			return nil, errors.New("请在独立的 Host 字段设置主机")
		case "content-length", "transfer-encoding", "trailer":
			return nil, errors.New("正文长度按实际字节计算；此工具不模拟传输分帧或尾部字段")
		}
		req.Header.Add(h.Name, h.Value)
	}
	return req, nil
}

type SingleDecision struct {
	Target    string `json:"target"`
	Revision  int64  `json:"revision"`
	Peer      string `json:"peer"`
	ClientIP  string `json:"client_ip"`
	Outcome   string `json:"outcome"`
	BodyBytes int64  `json:"body_bytes"`
	Result    Result `json:"result"`
}

func (r *Runtime) AssessSingle(ctx context.Context, s *Snapshot, in SingleRequest) (SingleDecision, error) {
	var out SingleDecision
	req, err := in.build(ctx, s.Config)
	if err != nil {
		return out, err
	}
	preview := &Snapshot{Config: CloneConfig(s.Config), Secrets: s.Secrets, Preview: true}
	if err = r.Prepare(preview); err != nil {
		return out, err
	}
	ip, err := ExtractClientIP(req, preview.Config.Policy.Network, preview.Compiled.Proxies)
	if err != nil {
		return out, err
	}
	req = WithTrustedExternalScheme(req, forwardedScheme(preview, req))
	c := NewRequestContext(req, PortID(s.Config.Port), ip, s.Config.Policy.RuleSet.View)
	c.DryRun = true
	if err = ctx.Err(); err != nil {
		return out, err
	}
	bodyBytes := req.ContentLength
	result := preview.Compiled.Pipeline.Evaluate(c)
	return SingleDecision{AssessmentTarget(s.Config), s.Config.Revision, req.RemoteAddr, ip.String(), assessmentOutcome(result, false), bodyBytes, result}, nil
}
