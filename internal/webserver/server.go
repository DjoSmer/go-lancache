package webserver

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"

	"go-lancache/internal/config"
)

type Server struct {
	certs  Certs
	config config.Config
	http   *http.Server
	https  *http.Server
	errors chan error
}

func NewServer(config config.Config) *Server {
	return &Server{
		config: config,
		certs:  *NewCerts(config.Ca),
		errors: make(chan error, 2),
	}
}

func (s *Server) Run(ctx context.Context, handler http.Handler) <-chan error {
	log := s.log("Run")

	s.http = &http.Server{
		Addr:    s.config.ServerAddress.Http,
		Handler: handler,
	}

	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		GetCertificate:     s.certs.GetCertificate,
	}

	s.https = &http.Server{
		Addr:      s.config.ServerAddress.Https,
		Handler:   handler,
		TLSConfig: tlsConfig,
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		wg.Done()
		log.Info(fmt.Sprintf("HTTP listening on %s", s.config.ServerAddress.Http))
		err := s.http.ListenAndServe()
		s.isError("http", err)
	}()

	go func() {
		wg.Done()
		log.Info(fmt.Sprintf("HTTPS listening on %s", s.config.ServerAddress.Https))
		err := s.https.ListenAndServeTLS("", "")
		s.isError("https", err)
	}()

	wg.Wait()

	return s.errors
}

func (s *Server) Shutdown(ctx context.Context) {
	s.http.Shutdown(ctx)
	s.https.Shutdown(ctx)
}

func (s *Server) isError(protocol string, err error) {
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		s.errors <- fmt.Errorf("Protocol %s: %w", protocol, err)
	}
}

func (s *Server) log(name string) *slog.Logger {
	op := "WebServer." + name
	return slog.Default().With(slog.String("op", op))
}
