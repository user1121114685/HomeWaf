package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	requestmodel "HomeWaf/internal/model/common/request"
	"HomeWaf/internal/model/common/response"
	"HomeWaf/internal/transporttest"
	"github.com/gin-gonic/gin"
)

type zeroBody struct{ read int64 }

func (z *zeroBody) Read(p []byte) (int, error) { clear(p); z.read += int64(len(p)); return len(p), nil }

func TestSecApiBoundsUnknownLengthAndPreservesStreamingUpload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	reached := false
	handler := func(c *gin.Context) { reached = true; c.Status(http.StatusNoContent) }
	r.POST("/probe", SecApi(), handler)
	const upload = "/api/v1/iplocation/upload"
	r.POST(upload, SecApi(), handler)
	id := transporttest.Session(t)
	stream := &zeroBody{}
	req := httptest.NewRequest(http.MethodPost, "/probe", io.LimitReader(stream, 40<<20))
	req.ContentLength = -1
	req.Header.Set(response.HeaderKeyID, id)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusRequestEntityTooLarge || reached || stream.read > (32<<20)+1 {
		t.Fatalf("unbounded transport read: status %d, reached %v, read %d", w.Code, reached, stream.read)
	}
	stream = &zeroBody{}
	req = httptest.NewRequest(http.MethodPost, upload, io.LimitReader(stream, 100<<20))
	req.Header.Set(response.HeaderKeyID, id)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=fixture")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent || !reached || stream.read != 0 {
		t.Fatal("multipart upload buffered by decryption middleware")
	}
	reached = false
	req = httptest.NewRequest(http.MethodPost, "/probe", nil)
	req.Header.Set(response.HeaderKeyID, id)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=fixture")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnsupportedMediaType || reached {
		t.Fatal("multipart accepted outside upload route")
	}
}

func TestPaginationRejectsUnboundedAndNegativeValues(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, body := range []string{`{"pageSize":-1}`, `{"pageSize":1001}`, `{"pageIndex":-1}`, `{"pageIndex":1000001}`} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		var req requestmodel.PageInfo
		if err := c.ShouldBindJSON(&req); err == nil {
			t.Fatalf("accepted unbounded pagination %s", body)
		}
	}
}
