// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package wafenginecore

import (
	"HomeWaf/internal/bodyinspect"
	"HomeWaf/wafenginecore/wafhttpcore"
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// 替换Body字符串内容
func (waf *WafEngine) ReplaceBodyContent(r *http.Request, oldStrings []string, newString string) error {
	if r.Header.Get("Content-Encoding") != "" {
		return errors.New("encoded request replacement is unavailable")
	}
	bodyByte, err := bodyinspect.ReadPrefix(r, 1<<20)
	if err != nil {
		return err
	}
	if len(bodyByte) > 1<<20 {
		return errors.New("request replacement limit exceeded")
	}
	for _, oldString := range oldStrings {
		if oldString == "" {
			continue
		}
		if len(bodyByte)+bytes.Count(bodyByte, []byte(oldString))*max(0, len(newString)-len(oldString)) > 1<<20 {
			return errors.New("request replacement expansion limit exceeded")
		}
		bodyByte = bytes.ReplaceAll(bodyByte, []byte(oldString), []byte(newString))
	}
	_ = r.Body.Close()
	r.Body = io.NopCloser(bytes.NewBuffer(bodyByte))
	r.ContentLength = int64(len(bodyByte))
	r.Header.Set("Content-Length", strconv.Itoa(len(bodyByte)))
	r.TransferEncoding = nil
	r.GetBody = nil
	r.Form = nil
	r.PostForm = nil
	return nil
}

// 替换URL字符串内容
func (waf *WafEngine) ReplaceURLContent(r *http.Request, oldStrings []string, newString string) error {
	// 深拷贝原始URL
	originalURL := r.URL
	newURL, _ := url.Parse(originalURL.String())

	// 处理路径部分
	decodedPath, err := url.PathUnescape(newURL.Path)
	if err == nil {
		for _, oldString := range oldStrings {
			if oldString != "" {
				decodedPath = strings.ReplaceAll(decodedPath, oldString, newString)
			}
		}
		newURL.Path = decodedPath
		newURL.RawPath = ""

	}

	rawQuery := newURL.RawQuery
	if rawQuery != "" {
		var queryParts []string
		// 拆分保留原始顺序
		pairs := strings.Split(rawQuery, "&")

		for _, pair := range pairs {
			// 分割键值对（考虑空值和包含多个=的情况）
			key, value, _ := strings.Cut(pair, "=")
			originalKey := key

			// 处理参数值
			if value != "" {
				// 保存原始编码值
				originalEncoded := value
				// 递归解码
				decodedValue := wafhttpcore.WafHttpCoreUrlEncode(value, 5)
				originalDecoded := decodedValue
				// 执行替换
				for _, oldString := range oldStrings {
					if oldString != "" {
						decodedValue = strings.ReplaceAll(decodedValue, oldString, newString)
					}
				}
				if decodedValue != originalDecoded {
					value = url.QueryEscape(decodedValue)
				} else {
					value = originalEncoded
				}
			}
			// 重新构建键值对
			if value == "" {
				queryParts = append(queryParts, key)
			} else {
				queryParts = append(queryParts, originalKey+"="+value)
			}
		}

		// 重新组合查询字符串
		newURL.RawQuery = strings.Join(queryParts, "&")
	} else {
		newURL.RawQuery = ""
	}

	*r.URL = *newURL

	r.RequestURI = newURL.RequestURI()
	return nil
}

// 替换Form字符串内容
func (waf *WafEngine) ReplaceFormContent(r *http.Request, oldString string, newString string) error {

	return nil
}
