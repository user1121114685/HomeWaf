package channels

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"HomeWaf/utils"
)

const requestTimeout = 20 * time.Second
const maxResponseBytes = 1024 * 1024

type Notifier struct {
	Kind                            string
	Config                          Config
	WebhookURL, Secret, AccessToken string
	client                          *http.Client
}

func New(kind, raw, webhookURL, secret, accessToken string) (*Notifier, error) {
	c, err := Parse(kind, raw)
	if err != nil {
		return nil, err
	}
	client := utils.SafeOutboundHTTPClient(requestTimeout)
	// Provider API endpoints should not redirect. This also prevents a redirect
	// from forwarding token-bearing bodies or custom authentication headers.
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return fmt.Errorf("通知接口不允许重定向") }
	return &Notifier{Kind: kind, Config: c, WebhookURL: webhookURL, Secret: secret, AccessToken: accessToken, client: client}, nil
}

func (n *Notifier) request(method, endpoint string, payload interface{}, headers map[string]string) (Config, error) {
	if ok, _ := utils.IsAllowedOutboundURL(endpoint); !ok {
		return nil, fmt.Errorf("通知接口地址不被允许，请检查出站地址允许清单")
	}
	var body io.Reader
	contentType := "application/json"
	if payload != nil {
		if form, ok := payload.(url.Values); ok {
			body = strings.NewReader(form.Encode())
			contentType = "application/x-www-form-urlencoded"
		} else {
			encoded, err := json.Marshal(payload)
			if err != nil {
				return nil, fmt.Errorf("通知报文编码失败")
			}
			body = bytes.NewReader(encoded)
		}
	}
	req, err := http.NewRequest(method, endpoint, body)
	if err != nil {
		return nil, fmt.Errorf("通知请求地址格式错误")
	}
	req.Header.Set("Content-Type", contentType)
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := n.client.Do(req)
	// Do not include url.Error or the raw provider response: both can contain
	// device keys, bot tokens and application credentials.
	if err != nil {
		return nil, fmt.Errorf("通知请求失败，请检查网络、证书及代理配置")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("通知接口返回 HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil || len(data) > maxResponseBytes {
		return nil, fmt.Errorf("通知接口响应读取失败或超出大小限制")
	}
	var result Config
	if json.Unmarshal(data, &result) != nil || result == nil {
		return nil, fmt.Errorf("通知接口返回了无效 JSON")
	}
	return result, nil
}

func requireCode(result Config, key string, expected int) error {
	v, exists := result[key].(float64)
	if !exists || v != float64(expected) {
		if exists {
			return fmt.Errorf("通知平台拒绝请求（%s=%g），请检查凭据、接收对象和权限", key, v)
		}
		return fmt.Errorf("通知接口响应缺少成功标记 %s", key)
	}
	return nil
}

func (n *Notifier) postCode(endpoint string, payload interface{}, headers map[string]string, code string, expected int) error {
	r, err := n.request("POST", endpoint, payload, headers)
	if err != nil {
		return err
	}
	return requireCode(r, code, expected)
}

func splitIDs(value string) []string {
	ids := []string{}
	for _, s := range strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == '，' || r == '\n' || r == ';' }) {
		if v := strings.TrimSpace(s); v != "" {
			ids = append(ids, v)
		}
	}
	return ids
}

func hmacSign(timestamp, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	_, _ = h.Write([]byte(timestamp + "\n" + secret))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func (n *Notifier) Send(title, content string) error {
	c := n.Config
	text := title + "\n\n" + content
	switch n.Kind {
	case "dingtalk":
		u, err := url.Parse(n.WebhookURL)
		if err != nil {
			return fmt.Errorf("钉钉 Webhook 地址无效")
		}
		if n.Secret != "" {
			ts := strconv.FormatInt(time.Now().UnixMilli(), 10)
			q := u.Query()
			q.Set("timestamp", ts)
			q.Set("sign", hmacSign(ts, n.Secret))
			u.RawQuery = q.Encode()
		}
		payload := Config{"msgtype": c.String("msgtype"), "at": Config{"isAtAll": c.Bool("atAll"), "atMobiles": splitIDs(c.String("atMobiles")), "atUserIds": splitIDs(c.String("atDingtalkIds"))}}
		if c.String("msgtype") == "text" {
			payload["text"] = Config{"content": text}
		} else {
			payload["markdown"] = Config{"title": title, "text": content}
		}
		return n.postCode(u.String(), payload, nil, "errcode", 0)
	case "wechatwork":
		payload := Config{"msgtype": c.String("msgType")}
		if c.String("msgType") == "text" {
			ids := splitIDs(c.String("atUserIds"))
			if c.Bool("atAll") {
				ids = append(ids, "@all")
			}
			payload["text"] = Config{"content": text, "mentioned_list": ids, "mentioned_mobile_list": splitIDs(c.String("atMobiles"))}
		} else {
			payload["markdown"] = Config{"content": content}
		}
		return n.postCode(n.WebhookURL, payload, nil, "errcode", 0)
	case "feishu":
		mentions := ""
		ids := splitIDs(c.String("atOpenIds"))
		if c.Bool("atAll") {
			ids = append(ids, "all")
		}
		for _, id := range ids {
			mentions += " <at user_id=\"" + html.EscapeString(id) + "\"></at>"
		}
		payload := Config{"msg_type": c.String("msgType")}
		if c.String("msgType") == "text" {
			payload["content"] = Config{"text": text + mentions}
		} else {
			payload["card"] = messageCard(c, title, content+mentions)
		}
		if n.Secret != "" {
			ts := strconv.FormatInt(time.Now().Unix(), 10)
			h := hmac.New(sha256.New, []byte(ts+"\n"+n.Secret))
			payload["timestamp"] = ts
			payload["sign"] = base64.StdEncoding.EncodeToString(h.Sum(nil))
		}
		r, err := n.request("POST", n.WebhookURL, payload, nil)
		if err != nil {
			return err
		}
		if _, exists := r["code"]; exists {
			return requireCode(r, "code", 0)
		}
		return requireCode(r, "StatusCode", 0)
	case "dingtalk_app":
		ids := splitIDs(c.String("userIds"))
		if len(ids) == 0 || len(ids) > 20 {
			return fmt.Errorf("钉钉接收用户数量须为 1–20")
		}
		r, err := n.request("POST", "https://api.dingtalk.com/v1.0/oauth2/accessToken", Config{"appKey": c.String("appKey"), "appSecret": c.String("appSecret")}, nil)
		if err != nil {
			return err
		}
		token := r.String("accessToken")
		if token == "" {
			return fmt.Errorf("钉钉获取 accessToken 失败，请检查应用凭据")
		}
		param := Config{"content": text}
		if c.String("msgKey") == "sampleMarkdown" {
			param = Config{"title": title, "text": content}
		}
		encoded, _ := json.Marshal(param)
		r, err = n.request("POST", "https://api.dingtalk.com/v1.0/robot/oToMessages/batchSend", Config{"robotCode": c.String("appKey"), "userIds": ids, "msgKey": c.String("msgKey"), "msgParam": string(encoded)}, map[string]string{"x-acs-dingtalk-access-token": token})
		if err != nil {
			return err
		}
		if r.String("processQueryKey") == "" {
			return fmt.Errorf("钉钉未返回消息任务标识，请检查机器人权限与接收人")
		}
		return nil
	case "feishu_app":
		r, err := n.request("POST", "https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal", Config{"app_id": c.String("appId"), "app_secret": c.String("appSecret")}, nil)
		if err != nil {
			return err
		}
		if err = requireCode(r, "code", 0); err != nil {
			return err
		}
		token := r.String("tenant_access_token")
		if token == "" {
			return fmt.Errorf("飞书未返回 tenant_access_token")
		}
		var msg interface{} = Config{"text": text}
		if c.String("msgType") == "interactive" {
			msg = messageCard(c, title, content)
		}
		encoded, _ := json.Marshal(msg)
		return n.postCode("https://open.feishu.cn/open-apis/im/v1/messages?receive_id_type="+url.QueryEscape(c.String("receiveIdType")), Config{"receive_id": c.String("receiveId"), "msg_type": c.String("msgType"), "content": string(encoded)}, map[string]string{"Authorization": "Bearer " + token}, "code", 0)
	case "wechatwork_app":
		base := strings.TrimRight(c.String("customizeAPI"), "/")
		r, err := n.request("GET", base+"/cgi-bin/gettoken?"+url.Values{"corpid": {c.String("corpID")}, "corpsecret": {c.String("secret")}}.Encode(), nil, nil)
		if err != nil {
			return err
		}
		if err = requireCode(r, "errcode", 0); err != nil {
			return err
		}
		token := r.String("access_token")
		if token == "" {
			return fmt.Errorf("企业微信未返回 access_token")
		}
		r, err = n.request("POST", base+"/cgi-bin/message/send?access_token="+url.QueryEscape(token), Config{"touser": c.String("toUser"), "toparty": c.String("toParty"), "totag": c.String("toTag"), "agentid": c.Int("agentID"), "msgtype": "text", "text": Config{"content": text}}, nil)
		if err != nil {
			return err
		}
		if err = requireCode(r, "errcode", 0); err != nil {
			return err
		}
		if r.String("invaliduser") != "" || r.String("invalidparty") != "" || r.String("invalidtag") != "" || r.String("unlicenseduser") != "" {
			return fmt.Errorf("企业微信部分接收对象无效或未获许可，请检查接收范围")
		}
		return nil
	case "bark":
		payload := Config{"title": title, "body": content, "isArchive": 1}
		for _, key := range []string{"group", "icon", "sound", "badge", "url", "level", "call"} {
			if c.String(key) != "" {
				payload[key] = c.String(key)
			}
		}
		if c.String("autoCopy") != "" {
			payload["autoCopy"] = 1
			payload["copy"] = strings.NewReplacer("[title]", title, "[content]", content).Replace(c.String("autoCopy"))
		}
		body, err := barkPayload(c, payload)
		if err != nil {
			return err
		}
		return n.postCode(c.String("server"), body, nil, "code", 200)
	case "telegram":
		return n.sendTelegram(text)
	case "pushplus":
		payload := Config{"token": c.String("token"), "title": title, "content": content}
		for _, key := range []string{"topic", "template"} {
			if c.String(key) != "" {
				payload[key] = c.String(key)
			}
		}
		if c.String("website") == "www.pushplus.plus" {
			for _, key := range []string{"channel", "webhook", "callbackUrl"} {
				if c.String(key) != "" {
					payload[key] = c.String(key)
				}
			}
			if c.Int("validTime") > 0 {
				payload["timestamp"] = time.Now().UnixMilli() + int64(c.Int("validTime"))*1000
			}
		}
		return n.postCode("https://"+c.String("website")+"/send", payload, nil, "code", 200)
	case "gotify":
		r, err := n.request("POST", c.String("webServer"), Config{"title": title, "message": content, "priority": c.Int("priority")}, nil)
		if err != nil {
			return err
		}
		if _, ok := r["id"].(float64); !ok {
			return fmt.Errorf("Gotify 未返回消息 ID")
		}
		return nil
	case "serverchan":
		if !regexp.MustCompile(`^[A-Za-z0-9]+$`).MatchString(n.AccessToken) {
			return fmt.Errorf("Server酱 SendKey 格式无效")
		}
		endpoint := "https://sctapi.ftqq.com/" + n.AccessToken + ".send"
		if strings.HasPrefix(n.AccessToken, "sctp") {
			parts := regexp.MustCompile(`^sctp(\d+)t`).FindStringSubmatch(n.AccessToken)
			if len(parts) != 2 {
				return fmt.Errorf("Server酱 SendKey 格式无效")
			}
			endpoint = "https://" + parts[1] + ".push.ft07.com/send/" + n.AccessToken + ".send"
		}
		titleRunes := []rune(title)
		if len(titleRunes) > 32 {
			title = string(titleRunes[:32])
		}
		if len(content) > 32768 {
			content = strings.ToValidUTF8(content[:32768], "")
		}
		return n.postCode(endpoint, url.Values{"title": {title}, "desp": {content}, "channel": {c.String("channel")}, "openid": {c.String("openid")}}, nil, "code", 0)
	case "socket":
		return n.sendSocket(title, content)
	}
	return fmt.Errorf("不支持的通知渠道")
}

func messageCard(c Config, title, content string) interface{} {
	if raw := c.String("messageCard"); raw != "" {
		var card interface{}
		if json.Unmarshal([]byte(raw), &card) == nil {
			return replaceCardStrings(card, strings.NewReplacer("{{MSG_TITLE}}", title, "{{MSG_CONTENT}}", content))
		}
	}
	return Config{"header": Config{"title": Config{"tag": "plain_text", "content": title}}, "elements": []interface{}{Config{"tag": "markdown", "content": content}}}
}
func replaceCardStrings(value interface{}, replacer *strings.Replacer) interface{} {
	switch v := value.(type) {
	case string:
		return replacer.Replace(v)
	case []interface{}:
		for i, item := range v {
			v[i] = replaceCardStrings(item, replacer)
		}
	case map[string]interface{}:
		for key, item := range v {
			v[key] = replaceCardStrings(item, replacer)
		}
	}
	return value
}

func (n *Notifier) sendTelegram(text string) error {
	c := n.Config
	if strings.ContainsAny(c.String("apiToken"), "/?# \r\n") {
		return fmt.Errorf("Telegram Bot Token 格式错误")
	}
	endpoint := strings.TrimRight(c.String("apiBase"), "/") + "/bot" + c.String("apiToken") + "/sendMessage"
	// Escape untrusted WAF event text before enabling Telegram parse modes. Split
	// before escaping so entities and UTF-8 characters are never cut in half.
	runes := []rune(text)
	for len(runes) > 0 {
		count := min(len(runes), 2000)
		chunk := string(runes[:count])
		runes = runes[count:]
		payload := Config{"chat_id": c.String("chatId"), "text": chunk, "disable_web_page_preview": true}
		mode := c.String("parseMode")
		if mode == "HTML" {
			payload["text"] = html.EscapeString(chunk)
			payload["parse_mode"] = mode
		}
		if mode == "MarkdownV2" {
			var b strings.Builder
			for _, r := range chunk {
				if strings.ContainsRune("_*[]()~`>#+-=|{}.!\\", r) {
					b.WriteByte('\\')
				}
				b.WriteRune(r)
			}
			payload["text"] = b.String()
			payload["parse_mode"] = mode
		}
		if c.Int("messageThreadId") > 0 {
			payload["message_thread_id"] = c.Int("messageThreadId")
		}
		var result Config
		var err error
		if c.String("method") == "GET" {
			q := url.Values{}
			for key, value := range payload {
				q.Set(key, fmt.Sprint(value))
			}
			result, err = n.request("GET", endpoint+"?"+q.Encode(), nil, nil)
		} else {
			result, err = n.request("POST", endpoint, payload, nil)
		}
		if err != nil {
			return err
		}
		if !result.Bool("ok") {
			return fmt.Errorf("Telegram 拒绝消息，请检查 Token、Chat ID、话题及权限")
		}
	}
	return nil
}
