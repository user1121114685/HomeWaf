// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

// HomeWaf adaptation of go-captcha v2.0.5: import namespace and maintained OpenType rendering.
/**
 * @Author Awen
 * @Date 2024/06/01
 * @Email wengaolng@gmail.com
 **/

package rotate

import (
	"image"
)

// Resources defines the resources for the rotate CAPTCHA
type Resources struct {
	rangImages []image.Image
}

// NewResources .
func NewResources() *Resources {
	return &Resources{}
}

type Resource func(*Resources)

// WithImages is to set image
func WithImages(images []image.Image) Resource {
	return func(resources *Resources) {
		resources.rangImages = images
	}
}
