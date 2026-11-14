package channels

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"HomeWaf/global"
	"github.com/eclipse/paho.mqtt.golang/packets"
)

func allowHosts(t *testing.T, hosts string) {
	t.Helper()
	old := global.CurrentSettings().GCONFIG_OUTBOUND_ALLOWED_HOSTS
	global.UpdateSettings(func(c *global.RuntimeSettings) { c.GCONFIG_OUTBOUND_ALLOWED_HOSTS = hosts })
	t.Cleanup(func() {
		global.UpdateSettings(func(c *global.RuntimeSettings) { c.GCONFIG_OUTBOUND_ALLOWED_HOSTS = old })
	})
}

func raw(c Config) string { b, _ := json.Marshal(c); return string(b) }

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestProviderProtocols(t *testing.T) {
	allowHosts(t, "notify.example,api.dingtalk.com,open.feishu.cn,qyapi.weixin.qq.com,www.pushplus.plus,sctapi.ftqq.com,123.push.ft07.com")
	cases := []struct {
		kind      string
		config    Config
		responses []string
		path      string
		inspect   func(*testing.T, *http.Request, Config)
	}{
		{"dingtalk", Config{"atAll": true, "atMobiles": "123,456"}, []string{`{"errcode":0}`}, "/hook", func(t *testing.T, r *http.Request, c Config) {
			if r.URL.Query().Get("sign") != hmacSign(r.URL.Query().Get("timestamp"), "sign-secret") {
				t.Error("incorrect DingTalk signature")
			}
			if c["at"].(map[string]interface{})["isAtAll"] != true {
				t.Error("mentions missing")
			}
		}},
		{"wechatwork", Config{"msgType": "text", "atAll": true}, []string{`{"errcode":0}`}, "/hook", func(t *testing.T, _ *http.Request, c Config) {
			if !strings.Contains(raw(c), "@all") {
				t.Error("mentions missing")
			}
		}},
		{"feishu", Config{}, []string{`{"code":0}`}, "/hook", func(t *testing.T, _ *http.Request, c Config) {
			h := hmac.New(sha256.New, []byte(c.String("timestamp")+"\nsign-secret"))
			if c.String("sign") != base64.StdEncoding.EncodeToString(h.Sum(nil)) {
				t.Error("incorrect Feishu signature")
			}
		}},
		{"dingtalk_app", Config{"appKey": "app-key", "appSecret": "app-secret", "userIds": "u1,u2"}, []string{`{"accessToken":"temporary-token"}`, `{"processQueryKey":"task1"}`}, "/v1.0/robot/oToMessages/batchSend", func(t *testing.T, r *http.Request, c Config) {
			if c.String("robotCode") != "app-key" || r.Header.Get("x-acs-dingtalk-access-token") != "temporary-token" {
				t.Error("wrong app authentication")
			}
			var p Config
			if json.Unmarshal([]byte(c.String("msgParam")), &p) != nil {
				t.Error("msgParam must be JSON string")
			}
		}},
		{"feishu_app", Config{"appId": "app", "appSecret": "secret", "receiveId": "u1", "receiveIdType": "open_id", "messageCard": `{"elements":[{"content":"{{MSG_CONTENT}}"}]}`}, []string{`{"code":0,"tenant_access_token":"tenant-token"}`, `{"code":0}`}, "/open-apis/im/v1/messages", func(t *testing.T, r *http.Request, c Config) {
			if r.URL.Query().Get("receive_id_type") != "open_id" || r.Header.Get("Authorization") != "Bearer tenant-token" {
				t.Error("wrong receiver or auth")
			}
			if !json.Valid([]byte(c.String("content"))) {
				t.Error("card substitution broke JSON")
			}
		}},
		{"wechatwork_app", Config{"corpID": "corp", "agentID": "1001", "secret": "secret"}, []string{`{"errcode":0,"access_token":"corp-token"}`, `{"errcode":0}`}, "/cgi-bin/message/send", func(t *testing.T, r *http.Request, c Config) {
			if r.URL.Query().Get("access_token") != "corp-token" || c.Int("agentid") != 1001 {
				t.Error("wrong corp payload")
			}
		}},
		{"bark", Config{"server": "https://notify.example/bark-device", "level": "timeSensitive", "group": "HomeWaf", "badge": "2", "autoCopy": "[title]"}, []string{`{"code":200}`}, "/bark-device", func(t *testing.T, _ *http.Request, c Config) {
			if c.String("level") != "timeSensitive" || c.String("copy") != "报警标题" {
				t.Error("Bark options missing")
			}
		}},
		{"telegram", Config{"apiBase": "https://notify.example", "apiToken": "123:token", "chatId": "-1001", "messageThreadId": "42", "parseMode": "HTML"}, []string{`{"ok":true}`}, "/bot123:token/sendMessage", func(t *testing.T, _ *http.Request, c Config) {
			if c.Int("message_thread_id") != 42 || !strings.Contains(c.String("text"), "&lt;") {
				t.Error("Telegram escaping or thread missing")
			}
		}},
		{"pushplus", Config{"token": "token", "topic": "ops", "validTime": "60"}, []string{`{"code":200}`}, "/send", func(t *testing.T, _ *http.Request, c Config) {
			if c.String("topic") != "ops" || c["timestamp"] == nil {
				t.Error("PushPlus options missing")
			}
		}},
		{"gotify", Config{"webServer": "https://notify.example/message?token=gotify-token", "priority": "8"}, []string{`{"id":10}`}, "/message", func(t *testing.T, r *http.Request, c Config) {
			if r.URL.Query().Get("token") != "gotify-token" || c.Int("priority") != 8 {
				t.Error("Gotify options missing")
			}
		}},
		{"serverchan", Config{"channel": "9|66", "openid": "copy-to"}, []string{`{"code":0}`}, "/SCTkey.send", func(t *testing.T, _ *http.Request, c Config) {
			if c.String("channel") != "9|66" || c.String("openid") != "copy-to" {
				t.Error("ServerChan options missing")
			}
		}},
	}
	for _, test := range cases {
		t.Run(test.kind, func(t *testing.T) {
			n, err := New(test.kind, raw(test.config), "https://notify.example/hook", "sign-secret", "SCTkey")
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			n.client.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
				var payload Config
				if strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
					_ = r.ParseForm()
					payload = Config{}
					for key, vs := range r.PostForm {
						payload[key] = vs[0]
					}
				} else if r.Body != nil {
					if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
						t.Fatal(err)
					}
				}
				if calls == len(test.responses)-1 {
					if r.URL.Path != test.path {
						t.Fatalf("path %s", r.URL.Path)
					}
					test.inspect(t, r, payload)
				}
				if calls >= len(test.responses) {
					t.Fatal("unexpected request")
				}
				response := test.responses[calls]
				calls++
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(response)), Header: make(http.Header)}, nil
			})
			if err := n.Send("报警标题", "恶意输入 <b> \"quoted\"\n\\path [msg]"); err != nil {
				t.Fatal(err)
			}
			if calls != len(test.responses) {
				t.Fatal("missing request")
			}
		})
	}
}

func TestRejectsFailedOrMalformedProviderResponses(t *testing.T) {
	allowHosts(t, "notify.example")
	for _, test := range []struct {
		body   string
		status int
	}{
		{`{"code":400,"message":"secret-value"}`, 200}, {`{}`, 200}, {`{"code":"200"}`, 200}, {`<html>bad gateway</html>`, 200}, {`{"code":200}`, 500}, {strings.Repeat("x", maxResponseBytes+1), 200},
	} {
		n, err := New("bark", `{"server":"https://notify.example/secret-value"}`, "", "", "")
		if err != nil {
			t.Fatal(err)
		}
		n.client.Transport = roundTrip(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: test.status, Body: io.NopCloser(strings.NewReader(test.body)), Header: make(http.Header)}, nil
		})
		err = n.Send("title", "content")
		if err == nil {
			t.Fatal("failure accepted")
		}
		if strings.Contains(err.Error(), "secret-value") {
			t.Fatal("secret in error")
		}
	}
}

func TestSecretsRoundTripAndValidation(t *testing.T) {
	rawConfig := `{"apiToken":"private-token","chatId":"123"}`
	masked, flags := MaskConfig("telegram", rawConfig)
	if strings.Contains(masked, "private-token") || !flags["apiToken"] {
		t.Fatal("token disclosed")
	}
	merged, err := MergeSecrets("telegram", masked, rawConfig, "__SAMWAF_CLEAR__")
	if err != nil {
		t.Fatal(err)
	}
	c, err := Parse("telegram", merged)
	if err != nil || c.String("apiToken") != "private-token" {
		t.Fatal("empty edit lost secret", err)
	}
	cleared, err := MergeSecrets("telegram", `{"apiToken":"__SAMWAF_CLEAR__","chatId":"123"}`, rawConfig, "__SAMWAF_CLEAR__")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Parse("telegram", cleared); err == nil {
		t.Fatal("cleared required token accepted")
	}
	changed, err := MergeSecrets("telegram", `{"apiToken":"new-token","chatId":"123"}`, rawConfig, "__SAMWAF_CLEAR__")
	if err != nil || !strings.Contains(changed, "new-token") {
		t.Fatal("replacement lost")
	}
	masked, flags = MaskConfig("bark", `{"server":"https://example.com/secret-device","key":"encryption-secret"}`)
	if strings.Contains(masked, "secret") || !flags["server"] || !flags["key"] {
		t.Fatal("Bark secrets disclosed")
	}
	for _, bad := range []string{`null`, `[]`, `{"apiToken":{},"chatId":"1"}`, `{"apiToken":"x","chatId":"1","method":"DELETE"}`, `{"apiToken":"x","chatId":"1","messageThreadId":1.5}`} {
		if _, err := Parse("telegram", bad); err == nil {
			t.Fatalf("invalid config accepted: %s", bad)
		}
	}
	if _, err = Parse("unknown", "{}"); err == nil {
		t.Fatal("unknown provider accepted")
	}
}

func TestOutboundAndRedirectProtection(t *testing.T) {
	allowHosts(t, "")
	for _, endpoint := range []string{"http://127.0.0.1/message", "http://169.254.169.254/latest/meta-data", "file:///etc/passwd"} {
		if _, err := New("gotify", raw(Config{"webServer": endpoint}), "", "", ""); err == nil {
			t.Fatalf("unsafe target accepted: %s", endpoint)
		}
	}
	allowHosts(t, "127.0.0.1")
	hit := false
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hit = true; fmt.Fprint(w, `{"code":200}`) }))
	defer destination.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, destination.URL, http.StatusTemporaryRedirect)
	}))
	defer redirect.Close()
	n, err := New("bark", raw(Config{"server": redirect.URL}), "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if err = n.Send("title", "body"); err == nil || hit {
		t.Fatal("redirect was followed")
	}
}

func TestBarkEncryption(t *testing.T) {
	for _, mode := range []string{"CBC/PKCS7Padding", "ECB/PKCS7Padding", "GCM/NoPadding"} {
		t.Run(mode, func(t *testing.T) {
			c := Config{"transformation": "AES128/" + mode, "key": "0123456789abcdef"}
			payload := Config{"title": "中文", "body": "告警\n内容"}
			encoded, err := barkPayload(c, payload)
			if err != nil {
				t.Fatal(err)
			}
			form := encoded.(url.Values)
			encrypted, err := base64.StdEncoding.DecodeString(form.Get("ciphertext"))
			if err != nil {
				t.Fatal(err)
			}
			block, _ := aes.NewCipher([]byte(c.String("key")))
			iv := []byte(form.Get("iv"))
			var plain []byte
			switch {
			case strings.HasPrefix(mode, "GCM"):
				gcm, _ := cipher.NewGCM(block)
				plain, err = gcm.Open(nil, iv, encrypted, nil)
			case strings.HasPrefix(mode, "CBC"):
				plain = make([]byte, len(encrypted))
				cipher.NewCBCDecrypter(block, iv).CryptBlocks(plain, encrypted)
			default:
				plain = make([]byte, len(encrypted))
				for i := 0; i < len(plain); i += aes.BlockSize {
					block.Decrypt(plain[i:i+aes.BlockSize], encrypted[i:i+aes.BlockSize])
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(mode, "GCM") {
				plain = plain[:len(plain)-int(plain[len(plain)-1])]
			}
			var decoded Config
			if json.Unmarshal(plain, &decoded) != nil || decoded.String("body") != payload.String("body") {
				t.Fatal("encryption roundtrip failed")
			}
		})
	}
}

func TestSocketTCPAndUDP(t *testing.T) {
	allowHosts(t, "127.0.0.1")
	for _, method := range []string{"TCP", "UDP"} {
		t.Run(method, func(t *testing.T) {
			var address string
			got := make(chan []byte, 1)
			if method == "TCP" {
				listener, err := net.Listen("tcp", "127.0.0.1:0")
				if err != nil {
					t.Fatal(err)
				}
				defer listener.Close()
				address = listener.Addr().String()
				go func() {
					conn, err := listener.Accept()
					if err != nil {
						return
					}
					defer conn.Close()
					_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
					buf := make([]byte, 4096)
					count, _ := conn.Read(buf)
					got <- buf[:count]
					_, _ = conn.Write([]byte("O"))
					_, _ = conn.Write([]byte("K\n"))
				}()
			} else {
				listener, err := net.ListenPacket("udp", "127.0.0.1:0")
				if err != nil {
					t.Fatal(err)
				}
				defer listener.Close()
				address = listener.LocalAddr().String()
				go func() {
					_ = listener.SetDeadline(time.Now().Add(3 * time.Second))
					buf := make([]byte, 4096)
					count, peer, err := listener.ReadFrom(buf)
					if err != nil {
						return
					}
					got <- buf[:count]
					_, _ = listener.WriteTo([]byte("OK"), peer)
				}()
			}
			host, port, _ := net.SplitHostPort(address)
			n, err := New("socket", raw(Config{"method": method, "address": host, "port": port, "response": "OK"}), "", "", "")
			if err != nil {
				t.Fatal(err)
			}
			if err = n.Send("中文标题", "a\"\n\\b [title]"); err != nil {
				t.Fatal(err)
			}
			select {
			case payload := <-got:
				var c Config
				if json.Unmarshal(payload, &c) != nil || c.String("msg") != "a\"\n\\b [title]" {
					t.Fatalf("unsafe template: %s", payload)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("no packet")
			}
		})
	}
}

func TestMQTTPublishQoS1(t *testing.T) {
	allowHosts(t, "127.0.0.1")
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	result := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			result <- err
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(4 * time.Second))
		packet, err := packets.ReadPacket(conn)
		if err != nil {
			result <- err
			return
		}
		connect, ok := packet.(*packets.ConnectPacket)
		if !ok || connect.Username != "user" || string(connect.Password) != "pass" {
			result <- fmt.Errorf("bad CONNECT")
			return
		}
		ack := packets.NewControlPacket(packets.Connack).(*packets.ConnackPacket)
		if err = ack.Write(conn); err != nil {
			result <- err
			return
		}
		packet, err = packets.ReadPacket(conn)
		if err != nil {
			result <- err
			return
		}
		publish, ok := packet.(*packets.PublishPacket)
		if !ok || publish.TopicName != "waf/alerts" || publish.Qos != 1 || !publish.Retain || !bytes.Contains(publish.Payload, []byte("告警")) {
			result <- fmt.Errorf("bad PUBLISH")
			return
		}
		puback := packets.NewControlPacket(packets.Puback).(*packets.PubackPacket)
		puback.MessageID = publish.MessageID
		result <- puback.Write(conn)
		_, _ = packets.ReadPacket(conn)
	}()
	host, port, _ := net.SplitHostPort(listener.Addr().String())
	n, err := New("socket", raw(Config{"method": "MQTT", "address": host, "port": port, "username": "user", "password": "pass", "outMessageTopic": "waf/alerts", "qos": "1", "retained": true}), "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if err = n.Send("告警", "内容"); err != nil {
		t.Fatal(err)
	}
	if err = <-result; err != nil {
		t.Fatal(err)
	}
}

func TestSocketCannotDialInternalWithoutAllowlist(t *testing.T) {
	allowHosts(t, "")
	n := Notifier{Config: Config{"address": "127.0.0.1", "port": "1234", "method": "TCP", "msgTemplate": "[msg]"}}
	if err := n.sendSocket("title", "content"); err == nil {
		t.Fatal("raw socket bypassed allowlist")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := waitToken(ctx, neverToken{}); err == nil {
		t.Fatal("context cancellation ignored")
	}
}

type neverToken struct{}

func (neverToken) Wait() bool                     { return false }
func (neverToken) WaitTimeout(time.Duration) bool { return false }
func (neverToken) Done() <-chan struct{}          { return make(chan struct{}) }
func (neverToken) Error() error                   { return nil }
