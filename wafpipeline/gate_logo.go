package wafpipeline

import (
	"bytes"
	"encoding/base64"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"strings"
)

// Logos are embedded raster data: no remote fetches, SVG scripts or arbitrary
// HTML. Pixel and encoded byte limits apply before a page is published.
func validateGateLogo(raw string) error {
	if raw == "" {
		return nil
	}
	if len(raw) > 90000 {
		return errors.New("登录Logo超过64KiB")
	}
	format := ""
	payload := ""
	for _, kind := range []string{"png", "jpeg"} {
		prefix := "data:image/" + kind + ";base64,"
		if strings.HasPrefix(raw, prefix) {
			format = kind
			payload = strings.TrimPrefix(raw, prefix)
			break
		}
	}
	if format == "" {
		return errors.New("登录Logo仅接受内嵌PNG/JPEG")
	}
	data, err := base64.StdEncoding.Strict().DecodeString(payload)
	if err != nil || len(data) > 65536 {
		return errors.New("登录Logo数据无效或超过64KiB")
	}
	config, decoded, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || decoded != format || config.Width < 1 || config.Height < 1 || config.Width > 512 || config.Height > 512 {
		return errors.New("登录Logo必须是有效PNG/JPEG，宽高不超过512像素")
	}
	return nil
}
