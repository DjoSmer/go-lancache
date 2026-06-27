package webserver_test

import (
	"context"
	"crypto/tls"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"go-lancache/internal/config"
	"go-lancache/internal/webserver"

	"github.com/stretchr/testify/assert"
)

var cfg config.Config
var webServer *webserver.Server

func test_example_com(t *testing.T) {
	t.Helper()

	serverName := "example.com"
	conn, err := tls.Dial("tcp", cfg.ServerAddress.Https, &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         serverName,
	})
	assert.NoError(t, err)
	defer conn.Close()

	state := conn.ConnectionState()
	cert := state.PeerCertificates[0]

	assert.Equal(t, cert.Subject.CommonName, serverName)
	assert.Len(t, cert.DNSNames, 2)
	assert.Equal(t, cert.DNSNames[0], serverName)
}

func TestSNI_example_com(t *testing.T) {
	test_example_com(t)
}

func TestSNI_sub_example_com(t *testing.T) {
	test_example_com(t)

	parentServerName := "example.com"
	serverName := "sub." + parentServerName
	conn, err := tls.Dial("tcp", cfg.ServerAddress.Https, &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         serverName,
	})
	assert.NoError(t, err)
	defer conn.Close()

	state := conn.ConnectionState()
	cert := state.PeerCertificates[0]

	assert.Equal(t, cert.Subject.CommonName, parentServerName)
}

func TestMain(m *testing.M) {
	log := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))
	slog.SetDefault(log)

	tmpDir := os.TempDir()
	certFile := filepath.Join(tmpDir, "go-lancache-test-ca.pem")
	keyFile := filepath.Join(tmpDir, "go-lancache-test-key.pem")

	cfg = config.Load()
	cfg.ServerAddress.Http = ":880"
	cfg.ServerAddress.Https = ":4443"
	cfg.Ca.Cert = certFile
	cfg.Ca.Key = keyFile

	webServer = webserver.NewServer(cfg)
	webServer.Run(context.Background(), http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "hello")
	}))

	code := m.Run()
	webServer.Close()
	os.Exit(code)
}
