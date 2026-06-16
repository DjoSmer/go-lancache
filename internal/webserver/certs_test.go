package lancache_test

import (
	"crypto/tls"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"go-lancache/internal/config"
	"go-lancache/internal/lancache"

	"github.com/brianvoe/gofakeit/v7"
)

var caCfg config.CaConfig
var certLog *slog.Logger

func BenchmarkGetCertificate(b *testing.B) {
	certs := lancache.NewCerts(caCfg, certLog)

	var c *tls.Certificate
	var err error
	for b.Loop() {
		c, err = certs.GetCertificate(&tls.ClientHelloInfo{
			ServerName: gofakeit.DomainName(),
		})
	}
	_ = c
	_ = err
}

func init() {
	tmpDir := os.TempDir()
	certFile := filepath.Join(tmpDir, "go-lancache-test-ca.pem")
	keyFile := filepath.Join(tmpDir, "go-lancache-test-key.pem")

	caCfg = config.CaConfig{
		Cert: certFile,
		Key:  keyFile,
	}

	certLog = slog.New(slog.NewTextHandler(io.Discard, nil))
}
