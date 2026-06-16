package lancache

import (
	"crypto/tls"
	"crypto/x509"
	"log/slog"
	"os"
	"strings"
	"sync"

	"go-lancache/internal/config"
)

type Certs struct {
	mu    sync.RWMutex
	certs map[string]*tls.Certificate

	ca  tls.Certificate
	log *slog.Logger
}

func NewCerts(config config.CaConfig, log *slog.Logger) *Certs {
	var ca tls.Certificate

	_, err := os.Stat(config.Key)
	if err == nil {
		log.Info("Load CA cert", slog.String("cert", config.Cert))
		ca, err = tls.LoadX509KeyPair(config.Cert, config.Key)
		if err != nil {
			panic("CA cert cannot load: " + err.Error())
		}

		ca.Leaf, err = x509.ParseCertificate(ca.Certificate[0])
		if err != nil {
			panic("Error parsing builtin CA leaf: " + err.Error())
		}
	} else {
		log.Info("Create CA cert")
		ca, err = CreateCa(config.Cert, config.Key)
		if err != nil {
			panic("CA cert cannot create: " + err.Error())
		}
	}

	return &Certs{
		ca:    ca,
		log:   log,
		certs: make(map[string]*tls.Certificate),
	}
}

func (c *Certs) GetCertificate(chi *tls.ClientHelloInfo) (*tls.Certificate, error) {
	hostname := chi.ServerName
	log := c.getLog("GetCertificate").With(slog.String("hostname", hostname))

	cert, ok := c.Get(hostname)
	if ok {
		log.Debug("Get cert")
		return cert, nil
	}

	parentHostname, ok := c.removeTopLevel(hostname)
	if ok {
		cert, ok := c.Get(parentHostname)
		if ok {
			log.Debug("Get parent cert")
			return cert, nil
		}
	}

	log.Debug("Create cert")
	cert, err = c.CreateCert(hostname)
	if err != nil {
		return nil, err
	}

	return cert, nil
}

func (c *Certs) Get(hostname string) (*tls.Certificate, bool) {
	c.mu.RLock()
	cert, ok := c.certs[hostname]
	c.mu.RUnlock()
	return cert, ok
}

func (c *Certs) CreateCert(hostname string) (*tls.Certificate, error) {
	log := c.getLog("CreateCert").With(slog.String("hostname", hostname))

	cert, err := CreateCert(c.ca, hostname)
	if err != nil {
		log.Error("Create cert")
		return nil, err
	}

	c.mu.Lock()
	c.certs[hostname] = cert
	c.mu.Unlock()

	return cert, nil
}

func (c *Certs) removeTopLevel(hostname string) (string, bool) {
	parts := strings.Split(hostname, ".")

	if len(parts) <= 2 {
		return hostname, false
	}

	return strings.Join(parts[1:], "."), true
}

func (c *Certs) getLog(name string) *slog.Logger {
	op := "Certs." + name
	return c.log.With(slog.String("op", op))
}
