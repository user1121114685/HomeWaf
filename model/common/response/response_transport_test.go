// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package response

import (
	"HomeWaf/wafsec"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/curve25519"
)

func newCtx(t *testing.T, keyid string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/whatever", nil)
	if keyid != "" {
		req.Header.Set(HeaderKeyID, keyid)
	}
	c.Request = req
	return c, rec
}

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder) Response {
	t.Helper()
	var resp Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("响应不是合法 JSON: %v", err)
	}
	return resp
}

// 起一把可用的传输密钥并握手，返回 keyid
func setupSession(t *testing.T) string {
	t.Helper()
	if err := wafsec.InitCommKey(t.TempDir(), filepath.Join(t.TempDir(), "comm_key")); err != nil {
		t.Fatalf("初始化传输密钥失败: %v", err)
	}
	// 客户端那一侧只需要一个合法的 32 字节公钥，本用例不校验共享密钥内容
	keyid, _, err := wafsec.RegisterCommSession(clientPubForTest(t))
	if err != nil {
		t.Fatalf("握手失败: %v", err)
	}
	return keyid
}

// clientPubForTest 造一个合法的 X25519 客户端公钥（32 字节 base64）
func clientPubForTest(t *testing.T) string {
	t.Helper()
	var esk [32]byte
	if _, err := io.ReadFull(rand.Reader, esk[:]); err != nil {
		t.Fatal(err)
	}
	var epk [32]byte
	curve25519.ScalarBaseMult(&epk, &esk)
	return base64.StdEncoding.EncodeToString(epk[:])
}

func TestResultUsesV2WhenSessionValid(t *testing.T) {
	keyid := setupSession(t)
	c, rec := newCtx(t, keyid)

	OkWithData(map[string]string{"hello": "world"}, c)

	resp := decodeBody(t, rec)
	data, _ := resp.Data.(string)
	if !strings.HasPrefix(data, wafsec.TransportPrefixV2) {
		t.Fatalf("有会话密钥时没走 v2，实际: %.20s", data)
	}
	plain, gotKeyid, err := wafsec.TransportDecrypt(data)
	if err != nil {
		t.Fatalf("v2 响应解密失败: %v", err)
	}
	if gotKeyid != keyid {
		t.Fatalf("keyid 不符: %s", gotKeyid)
	}
	if !strings.Contains(string(plain), "world") {
		t.Fatalf("明文内容不符: %s", plain)
	}
}

func TestSessionRequiredForEncryptedResults(t *testing.T) {
	for _, id := range []string{"", "expired-session"} {
		c, rec := newCtx(t, id)
		OkWithData(map[string]string{"secret": "never-in-plaintext"}, c)
		reply := decodeBody(t, rec)
		if reply.Code != NEED_REHANDSHAKE || reply.Data != "" {
			t.Fatalf("unexpected reply: %#v", reply)
		}
	}
}

// 握手类接口的响应必须是明文——此时双方还没有共享密钥
func TestPlainHelpersDoNotEncrypt(t *testing.T) {
	c, rec := newCtx(t, "")
	OkWithPlainData(gin.H{"pub": "abc"}, c)

	resp := decodeBody(t, rec)
	m, ok := resp.Data.(map[string]interface{})
	if !ok || m["pub"] != "abc" {
		t.Fatalf("握手响应不是明文对象: %#v", resp.Data)
	}
}
