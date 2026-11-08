// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

// HomeWaf adaptation of go-captcha v2.0.5: import namespace and maintained OpenType rendering.
/**
 * @Author Awen
 * @Date 2024/06/01
 * @Email wengaolng@gmail.com
 **/

package rotate

import (
	"HomeWaf/internal/thirdparty/gocaptcha/base/option"
)

// defaultOptions .
func defaultOptions() Option {
	return func(opts *Options) {
		opts.imageSquareSize = 220
		opts.rangeAnglePos = []*option.RangeVal{
			{Min: 30, Max: 330},
		}

		opts.thumbImageAlpha = 1
		opts.rangeThumbImageSquareSize = []int{140, 150, 160, 170}
	}
}

// defaultResource .
func defaultResource() Resource {
	return func(resources *Resources) {
		// ...
	}
}
