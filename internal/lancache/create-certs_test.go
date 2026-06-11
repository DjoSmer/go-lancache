package lancache_test

import (
	"bytes"
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

	"go-lancache/internal/lancache"
)

func orFatal(t *testing.T, msg string, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(msg, err)
	}
}

type ConstantHandler string

func (h ConstantHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	_, _ = io.WriteString(w, string(h))
}

func testSignerX509(t *testing.T, ca tls.Certificate) {
	t.Helper()
	fmt.Println(ca.Leaf.Subject)
	cert, err := lancache.CreateCert(ca, "example.com")
	orFatal(t, "CreateCert", err)

	cert.Leaf, err = x509.ParseCertificate(cert.Certificate[0])
	orFatal(t, "ParseCertificate", err)

	certPool := x509.NewCertPool()
	certPool.AddCert(ca.Leaf)
	orFatal(t, "VerifyHostname", cert.Leaf.VerifyHostname("example.com"))
	orFatal(t, "CheckSignatureFrom", cert.Leaf.CheckSignatureFrom(ca.Leaf))

	_, err = cert.Leaf.Verify(x509.VerifyOptions{
		DNSName: "example.com",
		Roots:   certPool,
	})
	orFatal(t, "Verify", err)
}

func testSignerTLS(t *testing.T, ca tls.Certificate) {
	t.Helper()
	cert, err := lancache.CreateCert(ca, "localhost")
	orFatal(t, "CreateCert", err)

	cert.Leaf, err = x509.ParseCertificate(cert.Certificate[0])
	orFatal(t, "ParseCertificate", err)

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
	orFatal(t, "NewRequest", err)

	certPool := x509.NewCertPool()
	certPool.AddCert(ca.Leaf)
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{RootCAs: certPool},
	}
	resp, err := tr.RoundTrip(req)
	orFatal(t, "RoundTrip", err)

	txt, err := io.ReadAll(resp.Body)
	orFatal(t, "io.ReadAll", err)
	if string(txt) != expected {
		t.Errorf("Expected '%s' got '%s'", expected, string(txt))
	}
}

func TestCreateCa(t *testing.T) {
	tmpDir := t.TempDir()
	certFile := filepath.Join(tmpDir, "go-lancache-test-ca.pem")
	keyFile := filepath.Join(tmpDir, "go-lancache-test-key.pem")

	ca1, err := lancache.CreateCa(certFile, keyFile)
	orFatal(t, "CreateCa", err)

	ca2, err := tls.LoadX509KeyPair(certFile, keyFile)
	orFatal(t, "LoadX509KeyPair", err)

	if !bytes.Equal(ca1.Certificate[0], ca2.Certificate[0]) {
		t.Error("CreateCa != LoadX509KeyPair")
	}
}

func TestCreateCertTls(t *testing.T) {
	ca, _ := lancache.CreateCa("", "")
	testSignerTLS(t, ca)
}

func TestCreateCertX509(t *testing.T) {
	ca, _ := lancache.CreateCa("", "")
	testSignerX509(t, ca)
}

func BenchmarkSignRsa(b *testing.B) {
	ca, _ := lancache.CreateCa("", "")
	var c *tls.Certificate
	var err error
	for b.Loop() {
		c, err = lancache.CreateCert(ca, "example.com")
	}
	_ = c
	_ = err
}
