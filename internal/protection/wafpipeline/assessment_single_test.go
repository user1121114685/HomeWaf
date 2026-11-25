package wafpipeline

import (
	"context"
	"io"
	"strings"
	"testing"
)

func singleFixture() SingleRequest {
	return SingleRequest{SourceIP: "2001:db8::35", SourcePort: 43210, Method: "PROPFIND", Scheme: "https", Version: "HTTP/2.0", Host: "alternate.example.test:8443", Path: "/files/a%2Fb", Query: "tag=a%2Bb&tag=b", Headers: []RequestHeader{{"X-Test", "one"}, {"x-test", "two"}, {"Origin", "https://other.example.test"}, {"Cookie", "test=demo"}, {"Authorization", "Bearer fixture"}}, Body: "AAEC/w==", BodyEncoding: "base64"}
}
func TestAssessmentSinglePreservesParsedHTTPInputs(t *testing.T) {
	_, s, _ := assessmentFixture()
	in := singleFixture()
	req, err := in.build(context.Background(), s.Config)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := io.ReadAll(req.Body)
	if req.URL.Host != "127.0.0.1:15000" || req.Host != in.Host || req.ProtoMajor != 2 || req.TLS == nil || req.Method != "PROPFIND" || req.RequestURI != "/files/a%2Fb?tag=a%2Bb&tag=b" || req.RemoteAddr != "[2001:db8::35]:43210" || string(data) != "\x00\x01\x02\xff" || req.ContentLength != 4 {
		t.Fatalf("request options did not reach the evaluator: %s %s %s %d", req.Method, req.RequestURI, req.RemoteAddr, len(data))
	}
	if strings.Join(req.Header.Values("X-Test"), "/") != "one/two" || req.Header.Get("Origin") != "https://other.example.test" || req.Header.Get("Cookie") != "test=demo" || req.Header.Get("Authorization") != "Bearer fixture" || req.UserAgent() != "" {
		t.Fatal("headers rewritten, injected or collapsed")
	}
	for _, edit := range []func(*SingleRequest){func(i *SingleRequest) { i.Path = "//other.test/a" }, func(i *SingleRequest) { i.Path = "/x?q=1" }, func(i *SingleRequest) { i.Version = "0" }, func(i *SingleRequest) { i.SourcePort = 0 }, func(i *SingleRequest) { i.Body = "!invalid" }, func(i *SingleRequest) { i.Method = "no space" }, func(i *SingleRequest) { i.Headers = []RequestHeader{{"Content-Length", "999"}} }, func(i *SingleRequest) { i.Headers = []RequestHeader{{"X-Test", "bad\r\nfield"}} }, func(i *SingleRequest) { i.BodyEncoding = "text"; i.Body = strings.Repeat("x", 65537) }} {
		v := singleFixture()
		edit(&v)
		if _, err := v.build(context.Background(), s.Config); err == nil {
			t.Fatal("invalid field accepted")
		}
	}
}
func TestAssessmentSingleUsesRealProxyTrustAndNoLiveCounters(t *testing.T) {
	r, s, controls := assessmentFixture()
	in := singleFixture()
	in.Headers = append(in.Headers, RequestHeader{"X-Forwarded-For", "192.0.2.47"})
	for _, trusted := range []bool{false, true} {
		want := in.SourceIP
		if trusted {
			s.Config.Policy.Network.TrustedProxies = []string{in.SourceIP + "/128"}
			want = "192.0.2.47"
		}
		out, err := r.AssessSingle(context.Background(), s, in)
		if err != nil {
			t.Fatal(err)
		}
		if out.ClientIP != want || out.BodyBytes != 4 || out.Result.ReqID == "" || len(out.Result.Trace) == 0 || s.Compiled != nil || r.Requests.Load() != 0 || len(controls.Rates.entries) != 0 {
			t.Fatal("lost source identity or modified live state", out.ClientIP)
		}
	}
}
