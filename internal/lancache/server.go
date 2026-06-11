package lancache

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
	certs   Certs
	config  config.Config
	log     *slog.Logger
	servers []*http.Server
}

func NewServer(config config.Config, log *slog.Logger) *Server {
	return &Server{
		log:     log,
		config:  config,
		certs:   *NewCerts(config.Ca, log),
		servers: make([]*http.Server, 2),
	}
}

func (s *Server) Run(ctx context.Context, handler http.Handler) {
	log := s.log.With(slog.String("op", "Server.Run"))

	s.servers[0] = &http.Server{
		Addr:    s.config.ServerAddress.Http,
		Handler: handler,
	}

	tlsConfig := &tls.Config{
		InsecureSkipVerify: false,
		GetCertificate:     s.certs.GetCertificate,
	}

	s.servers[1] = &http.Server{
		Addr:      s.config.ServerAddress.Https,
		Handler:   handler,
		TLSConfig: tlsConfig,
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		wg.Done()
		log.Info(fmt.Sprintf("HTTP listening on %s", s.config.ServerAddress.Http))
		err := s.servers[0].ListenAndServe()
		s.isError(err)
	}()

	go func() {
		wg.Done()
		log.Info(fmt.Sprintf("HTTPS listening on %s", s.config.ServerAddress.Https))
		err := s.servers[1].ListenAndServeTLS("", "")
		s.isError(err)
	}()

	//Waiting when goroutines run and servers start
	wg.Wait()
}

func (s *Server) Close() {
	for _, srv := range s.servers {
		srv.Close()
	}
}

func (s *Server) isError(err error) {
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		panic(err)
	}
}
