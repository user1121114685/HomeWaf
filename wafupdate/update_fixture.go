//go:build homewaf_update_fixture

package wafupdate

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"os"
)

func init() {
	path := os.Getenv("HOMEWAF_SMOKE_CA")
	if path == "" {
		return
	}
	pem, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(pem) {
		panic("invalid smoke-test publisher certificate")
	}
	transport := defaultHTTPClient.Transport.(*http.Transport)
	transport.TLSClientConfig = &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}
}
