package channels

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"HomeWaf/utils"
	"github.com/coder/websocket"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/simplifiedchinese"
)

func charset(name string) encoding.Encoding {
	switch name {
	case "GBK":
		return simplifiedchinese.GBK
	case "GB18030":
		return simplifiedchinese.GB18030
	case "ISO-8859-1":
		return charmap.ISO8859_1
	}
	return nil
}

func socketMessage(c Config, title, content string) []byte {
	template := c.String("msgTemplate")
	ts := strconv.FormatInt(time.Now().UnixMilli(), 10)
	values := map[string]string{"title": title, "content": content, "msg": content, "timestamp": ts, "sign": ""}
	if c.String("secret") != "" {
		values["sign"] = url.QueryEscape(hmacSign(ts, c.String("secret")))
	}
	isJSON := json.Valid([]byte(template))
	pairs := []string{}
	for _, key := range []string{"title", "content", "msg", "timestamp", "sign"} {
		value := values[key]
		if isJSON {
			encoded, _ := json.Marshal(value)
			value = string(encoded[1 : len(encoded)-1])
		} else {
			value = url.QueryEscape(value)
		}
		pairs = append(pairs, "["+key+"]", value)
	}
	return []byte(strings.NewReplacer(pairs...).Replace(template))
}

func (n *Notifier) sendSocket(title, content string) error {
	c := n.Config
	payload := socketMessage(c, title, content)
	if len(payload) > 256*1024 {
		return fmt.Errorf("Socket 消息超过 256 KiB")
	}
	if codec := charset(c.String("outCharset")); codec != nil {
		var err error
		payload, err = codec.NewEncoder().Bytes(payload)
		if err != nil {
			return fmt.Errorf("消息含有无法使用所选编码表示的字符")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()
	if c.String("method") == "MQTT" {
		return n.sendMQTT(ctx, payload)
	}
	if c.String("method") == "UDP" && len(payload) > 65507 {
		return fmt.Errorf("UDP 消息超过单个数据报限制")
	}
	conn, err := utils.DialAllowedOutbound(ctx, strings.ToLower(c.String("method")), net.JoinHostPort(c.String("address"), c.String("port")))
	if err != nil {
		return fmt.Errorf("Socket 连接失败，请检查地址、端口与出站允许清单")
	}
	defer conn.Close()
	deadline, _ := ctx.Deadline()
	_ = conn.SetDeadline(deadline)
	if c.String("method") == "UDP" {
		size, e := conn.Write(payload)
		if e != nil || size != len(payload) {
			return fmt.Errorf("UDP 消息发送失败")
		}
	} else {
		if _, err = io.Copy(conn, bytes.NewReader(payload)); err != nil {
			return fmt.Errorf("TCP 消息发送失败")
		}
	}
	if c.String("response") == "" {
		return nil
	}
	expected := []byte(c.String("response"))
	if codec := charset(c.String("inCharset")); codec != nil {
		expected, err = codec.NewEncoder().Bytes(expected)
		if err != nil {
			return fmt.Errorf("成功应答关键字与接收编码不兼容")
		}
	}
	buffer := make([]byte, 64*1024)
	received := []byte{}
	for len(received) < maxResponseBytes {
		count, e := conn.Read(buffer)
		received = append(received, buffer[:count]...)
		if bytes.Contains(received, expected) {
			return nil
		}
		if e != nil {
			break
		}
	}
	return fmt.Errorf("Socket 未收到匹配的成功应答")
}

func waitToken(ctx context.Context, token mqtt.Token) error {
	select {
	case <-ctx.Done():
		return fmt.Errorf("MQTT 请求超时")
	case <-token.Done():
		if token.Error() != nil {
			return fmt.Errorf("MQTT 请求失败，请检查服务、凭据和主题权限")
		}
		return nil
	}
}

func (n *Notifier) sendMQTT(ctx context.Context, payload []byte) error {
	c := n.Config
	u := url.URL{Scheme: c.String("uriType"), Host: net.JoinHostPort(c.String("address"), c.String("port"))}
	if u.Scheme == "ws" || u.Scheme == "wss" {
		u.Path = c.String("path")
	}
	options := mqtt.NewClientOptions().AddBroker(u.String()).SetClientID(c.Default("clientId", "homewaf-"+uuid.NewString())).SetUsername(c.String("username")).SetPassword(c.String("password")).SetAutoReconnect(false).SetConnectRetry(false).SetConnectTimeout(requestTimeout).SetWriteTimeout(requestTimeout).SetCleanSession(true)
	// All MQTT transports use our guarded dialer, including websocket handshakes.
	options.SetCustomOpenConnectionFn(func(endpoint *url.URL, _ mqtt.ClientOptions) (net.Conn, error) {
		if endpoint.Scheme == "ws" || endpoint.Scheme == "wss" {
			transport := utils.SafeOutboundDirectTransport(nil)
			client := &http.Client{Transport: transport, Timeout: requestTimeout, CheckRedirect: func(*http.Request, []*http.Request) error { return fmt.Errorf("MQTT 不允许重定向") }}
			ws, _, err := websocket.Dial(ctx, endpoint.String(), &websocket.DialOptions{HTTPClient: client, Subprotocols: []string{"mqtt"}})
			transport.CloseIdleConnections()
			if err != nil {
				return nil, err
			}
			ws.SetReadLimit(maxResponseBytes)
			return websocket.NetConn(ctx, ws, websocket.MessageBinary), nil
		}
		conn, err := utils.DialAllowedOutbound(ctx, "tcp", endpoint.Host)
		if err != nil {
			return nil, err
		}
		if endpoint.Scheme == "ssl" {
			tlsConn := tls.Client(conn, &tls.Config{ServerName: endpoint.Hostname(), MinVersion: tls.VersionTLS12})
			if err = tlsConn.HandshakeContext(ctx); err != nil {
				conn.Close()
				return nil, err
			}
			return tlsConn, nil
		}
		return conn, nil
	})
	client := mqtt.NewClient(options)
	if err := waitToken(ctx, client.Connect()); err != nil {
		client.Disconnect(0)
		return err
	}
	defer client.Disconnect(100)
	ack := make(chan bool, 1)
	if c.String("response") != "" {
		err := waitToken(ctx, client.Subscribe(c.String("inMessageTopic"), byte(c.Int("qos")), func(_ mqtt.Client, message mqtt.Message) {
			data := message.Payload()
			if len(data) > maxResponseBytes {
				return
			}
			if codec := charset(c.String("inCharset")); codec != nil {
				var err error
				data, err = codec.NewDecoder().Bytes(data)
				if err != nil {
					return
				}
			}
			// A retained acknowledgement belongs to an earlier message.
			if !message.Retained() && strings.Contains(string(data), c.String("response")) {
				select {
				case ack <- true:
				default:
				}
			}
		}))
		if err != nil {
			return err
		}
	}
	if err := waitToken(ctx, client.Publish(c.String("outMessageTopic"), byte(c.Int("qos")), c.Bool("retained"), payload)); err != nil {
		return err
	}
	if c.String("response") != "" {
		select {
		case <-ack:
			return nil
		case <-ctx.Done():
			return fmt.Errorf("MQTT 未收到匹配的成功应答")
		}
	}
	return nil
}
