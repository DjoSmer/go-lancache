package webserver_test

import (
	"crypto/tls"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"go-lancache/internal/config"
	"go-lancache/internal/webserver"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/stretchr/testify/assert"
)

var caCfg config.CaConfig

func init() {
	log := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	slog.SetDefault(log)

	tmpDir := os.TempDir()
	certFile := filepath.Join(tmpDir, "go-lancache-test-ca.pem")
	keyFile := filepath.Join(tmpDir, "go-lancache-test-key.pem")

	caCfg = config.CaConfig{
		Cert: certFile,
		Key:  keyFile,
	}
}

func TestGetCertificate(t *testing.T) {
	serverName := gofakeit.DomainName()
	certs := webserver.NewCerts(caCfg)
	cert, err := certs.GetCertificate(&tls.ClientHelloInfo{
		ServerName: serverName,
	})
	assert.NoError(t, err)
	assert.NoError(t, cert.Leaf.VerifyHostname(serverName))
}

func BenchmarkGetCertificate(b *testing.B) {
	certs := webserver.NewCerts(caCfg)

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
