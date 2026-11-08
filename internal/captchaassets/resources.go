// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package captchaassets

import (
	_ "embed"
	"golang.org/x/image/font/sfnt"
	"image"
	"image/color"
	"sync"
)

//go:embed LXGWWenKai-Regular.ttf
var fontBytes []byte
var once sync.Once
var loaded *sfnt.Font
var loadError error

func GetFont() (*sfnt.Font, error) {
	once.Do(func() { loaded, loadError = sfnt.Parse(fontBytes) })
	return loaded, loadError
}

// GetImages creates original procedural backgrounds; no third-party photographs are embedded.
func GetImages() ([]image.Image, error) {
	out := make([]image.Image, 4)
	for i := range out {
		img := image.NewNRGBA(image.Rect(0, 0, 320, 160))
		for y := 0; y < 160; y++ {
			for x := 0; x < 320; x++ {
				noise := (x*17 + y*31 + i*43) % 23
				img.SetNRGBA(x, y, color.NRGBA{R: uint8(185 + (x+i*7)%45), G: uint8(190 + (y+i*11)%42), B: uint8(210 + noise), A: 255})
			}
		}
		out[i] = img
	}
	return out, nil
}
