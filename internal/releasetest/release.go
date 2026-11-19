// Package releasetest supplies release metadata for isolated update tests.
package releasetest

import "strings"

func Metadata(version, description string) map[string]any {
	var assets []map[string]string
	for _, platform := range []string{"Windows_x86_64", "Linux_x86_64", "Linux_arm64", "Darwin_x86_64", "Darwin_arm64"} {
		extension := ".tar.gz"
		if platform == "Windows_x86_64" {
			extension = ".zip"
		}
		name := "HomeWaf_" + platform + "_" + strings.TrimPrefix(version, "v") + extension
		assets = append(assets, map[string]string{"name": name, "browser_download_url": "https://publisher.example/" + name, "digest": "sha256:" + strings.Repeat("0", 64)})
	}
	return map[string]any{"tag_name": version, "body": description, "assets": assets}
}
