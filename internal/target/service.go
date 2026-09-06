package target

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"go-lancache/internal/config"
)

var ErrTargetsEmpty = errors.New("Targets are nil")
var ErrSkipTarget = errors.New("Skip Target")

var defaultNoTargetUrls = []string{
	"\\.(json|exe|css|cur|js|jpe?g|gif|ico|png|svg|webp|webm|html|woff|woff2|xml)",
	"(authrootstl.cab|pinrulesstl.cab|disallowedcertstl.cab)$",
	"(releaselisting_.*|.version$)",
}

type TargetServiceRepository interface {
	FindById(ctx context.Context, id int) (Target, error)
	GetAll(ctx context.Context) ([]Target, error)
	Save(target Target) (Target, error)
}

type TargetService struct {
	config             config.TargetConfig
	repo               TargetServiceRepository
	targets            []Target
	noTargetUrlsRegexp []*regexp.Regexp
}

func NewTargetService(repo TargetServiceRepository, config config.TargetConfig) *TargetService {
	noTargetUrlsRegexp := make([]*regexp.Regexp, len(defaultNoTargetUrls))
	for i, noTargetUrl := range defaultNoTargetUrls {
		noTargetUrlsRegexp[i] = regexp.MustCompile(noTargetUrl)
	}

	return &TargetService{
		config:             config,
		repo:               repo,
		targets:            nil,
		noTargetUrlsRegexp: noTargetUrlsRegexp,
	}
}

func (s *TargetService) Get(cxt context.Context, domain string, userAgent string, url string) (Target, error) {
	log := s.log("Get")
	log.Debug("Run", slog.String("domain", domain), slog.String("userAgent", userAgent), slog.String("url", url))

	if s.targets == nil {
		return Target{}, ErrTargetsEmpty
	}

	for _, re := range s.noTargetUrlsRegexp {
		if re.MatchString(url) {
			return Target{}, ErrSkipTarget
		}
	}

	for _, target := range s.targets {
		if target.UserAgent != "" && target.UserAgent == userAgent {
			return target, nil
		}

		if target.Domain != "" && strings.HasSuffix(domain, target.Domain) {
			return target, nil
		}
	}

	return Target{}, ErrSkipTarget
}

func (s *TargetService) Load() {
	log := s.log("Load")

	targets, err := s.repo.GetAll(context.Background())
	if err != nil {
		log.Error("Cannot load target: " + err.Error())
		return
	}

	log.Debug(fmt.Sprintf("Loaded %d targets", len(targets)))
	s.targets = targets
}

func (s *TargetService) Import() {}

func (s *TargetService) log(name string) *slog.Logger {
	op := "TargetService." + name
	return slog.Default().With(slog.String("op", op))
}
