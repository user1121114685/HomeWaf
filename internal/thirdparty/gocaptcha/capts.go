// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

// HomeWaf adaptation of go-captcha v2.0.5: import namespace and maintained OpenType rendering.
/**
 * @Author Awen
 * @Date 2024/06/01
 * @Email wengaolng@gmail.com
 **/

package v2

import (
	"HomeWaf/internal/thirdparty/gocaptcha/click"
	"HomeWaf/internal/thirdparty/gocaptcha/rotate"
	"HomeWaf/internal/thirdparty/gocaptcha/slide"
)

// Version # of captcha
const Version = "2.0.4"

// NewClickBuilder .
func NewClickBuilder(opts ...click.Option) click.Builder {
	return click.NewBuilder(opts...)
}

// NewSlideBuilder .
func NewSlideBuilder(opts ...slide.Option) slide.Builder {
	return slide.NewBuilder(opts...)
}

// NewRotateBuilder .
func NewRotateBuilder(opts ...rotate.Option) rotate.Builder {
	return rotate.NewBuilder(opts...)
}
