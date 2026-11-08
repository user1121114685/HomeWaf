// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

// HomeWaf adaptation of go-captcha v2.0.5: import namespace and maintained OpenType rendering.
/**
 * @Author Awen
 * @Date 2024/06/01
 * @Email wengaolng@gmail.com
 **/

package rotate

import "HomeWaf/internal/thirdparty/gocaptcha/base/imagedata"

// CaptchaData defines the interface for rotate CAPTCHA data
type CaptchaData interface {
	GetData() *Block
	GetMasterImage() imagedata.PNGImageData
	GetThumbImage() imagedata.PNGImageData
}

// CaptData is the concrete implementation of the CaptchaData interface
type CaptData struct {
	block       *Block
	masterImage imagedata.PNGImageData
	thumbImage  imagedata.PNGImageData
}

var _ CaptchaData = (*CaptData)(nil)

// GetData is to get block
func (c CaptData) GetData() *Block {
	return c.block
}

// GetMasterImage is to get master image
func (c CaptData) GetMasterImage() imagedata.PNGImageData {
	return c.masterImage
}

// GetThumbImage is to get thumb image
func (c CaptData) GetThumbImage() imagedata.PNGImageData {
	return c.thumbImage
}
