package lancache_test

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
	"go-lancache/internal/lancache"
)

var cfg config.Config
var log *slog.Logger

type Suite struct {
	Server *lancache.Server
}

func New() *Suite {
	srv := lancache.NewServer(cfg, log)
	suite := &Suite{
		Server: srv,
	}

	srv.Run(context.Background(), suite)
	return suite
}

func (s *Suite) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	_, _ = io.WriteString(w, "hello")
}

func test_example_com(t *testing.T) *Suite {
	t.Helper()
	s := New()

	serverName := "example.com"
	conn, err := tls.Dial("tcp", cfg.ServerAddress.Https, &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         serverName,
	})
	orFatal(t, "Tls.Dial", err)
	defer conn.Close()

	state := conn.ConnectionState()
	cert := state.PeerCertificates[0]

	if cert.Subject.CommonName != serverName {
		t.Errorf("%s != %s", cert.Subject.CommonName, serverName)
	}

	if len(cert.DNSNames) == 0 || cert.DNSNames[0] != serverName {
		t.Errorf("DNSNames are empty or DNSNames[0] != %s", serverName)
	}

	return s
}

func TestSNI_example_com(t *testing.T) {
	s := test_example_com(t)
	defer s.Server.Close()
}

func TestSNI_sub_example_com(t *testing.T) {
	s := test_example_com(t)
	defer s.Server.Close()

	parentServerName := "example.com"
	serverName := "sub." + parentServerName
	conn, err := tls.Dial("tcp", "127.0.0.1:4443", &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         serverName,
	})
	orFatal(t, "Tls.Dial", err)
	defer conn.Close()

	state := conn.ConnectionState()

	cert := state.PeerCertificates[0]

	if cert.Subject.CommonName != parentServerName {
		t.Errorf("%s != %s", cert.Subject.CommonName, serverName)
	}
}

func init() {
	tmpDir := os.TempDir()
	certFile := filepath.Join(tmpDir, "go-lancache-test-ca.pem")
	keyFile := filepath.Join(tmpDir, "go-lancache-test-key.pem")

	cfg = *config.Load()
	cfg.ServerAddress.Http = ":880"
	cfg.ServerAddress.Https = ":4443"
	cfg.Ca.Cert = certFile
	cfg.Ca.Key = keyFile

	log = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
}
