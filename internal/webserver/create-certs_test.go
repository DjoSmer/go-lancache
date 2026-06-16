package webserver_test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"go-lancache/internal/webserver"

	"github.com/stretchr/testify/assert"
)

type ConstantHandler string

func (h ConstantHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	_, _ = io.WriteString(w, string(h))
}

func testSignerX509(t *testing.T, ca tls.Certificate) {
	t.Helper()
	fmt.Println(ca.Leaf.Subject)
	cert, err := webserver.CreateCert(ca, "example.com")
	assert.NoError(t, err)

	cert.Leaf, err = x509.ParseCertificate(cert.Certificate[0])
	assert.NoError(t, err)

	certPool := x509.NewCertPool()
	certPool.AddCert(ca.Leaf)
	assert.NoError(t, cert.Leaf.VerifyHostname("example.com"))
	assert.NoError(t, cert.Leaf.CheckSignatureFrom(ca.Leaf))

	_, err = cert.Leaf.Verify(x509.VerifyOptions{
		DNSName: "example.com",
		Roots:   certPool,
	})
	assert.NoError(t, err)
}

func testSignerTLS(t *testing.T, ca tls.Certificate) {
	t.Helper()
	cert, err := webserver.CreateCert(ca, "localhost")
	assert.NoError(t, err)

	cert.Leaf, err = x509.ParseCertificate(cert.Certificate[0])
	assert.NoError(t, err)

	expected := "key verifies with Go"
	server := httptest.NewUnstartedServer(ConstantHandler(expected))
	defer server.Close()

	server.TLS = &tls.Config{
		Certificates: []tls.Certificate{*cert, ca},
		MinVersion:   tls.VersionTLS12,
	}
	server.StartTLS()

	asLocalhost := strings.ReplaceAll(server.URL, "127.0.0.1", "localhost")
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, asLocalhost, nil)
	assert.NoError(t, err)

	certPool := x509.NewCertPool()
	certPool.AddCert(ca.Leaf)
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{RootCAs: certPool},
	}
	resp, err := tr.RoundTrip(req)
	assert.NoError(t, err)

	txt, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.Equal(t, string(txt), expected)
}

func TestCreateCa(t *testing.T) {
	tmpDir := t.TempDir()
	certFile := filepath.Join(tmpDir, "go-lancache-test-ca.pem")
	keyFile := filepath.Join(tmpDir, "go-lancache-test-key.pem")

	ca1, err := webserver.CreateCa(certFile, keyFile)
	assert.NoError(t, err)

	ca2, err := tls.LoadX509KeyPair(certFile, keyFile)
	assert.NoError(t, err)

	assert.Equal(t, ca1.Certificate[0], ca2.Certificate[0])
}

func TestCreateCertTls(t *testing.T) {
	ca, _ := webserver.CreateCa("", "")
	testSignerTLS(t, ca)
}

func TestCreateCertX509(t *testing.T) {
	ca, _ := webserver.CreateCa("", "")
	testSignerX509(t, ca)
}

func BenchmarkSignRsa(b *testing.B) {
	ca, _ := webserver.CreateCa("", "")
	var c *tls.Certificate
	var err error
	for b.Loop() {
		c, err = webserver.CreateCert(ca, "example.com")
	}
	_ = c
	_ = err
}
