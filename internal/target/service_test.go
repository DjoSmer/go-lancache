package target_test

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"go-lancache/internal/config"
	"go-lancache/internal/target"

	"github.com/stretchr/testify/assert"
)

type mockTargetRepository struct{}

func (r *mockTargetRepository) FindById(ctx context.Context, id int) (target.Target, error) {
	return target.Target{}, nil
}

func (r *mockTargetRepository) GetAll(ctx context.Context) ([]target.Target, error) {
	return []target.Target{
		{Id: 2, Code: "test", UserAgent: "Test", Domain: "test.com"},
		{Id: 1, Code: "steam", UserAgent: "Valve/Steam HTTP Client 1.0"},
		{Id: 4, Code: "example", UserAgent: "Example", Domain: "example.com"},
		{Id: 3, Code: "valve", Domain: "valve.com"},
	}, nil
}

func (r *mockTargetRepository) Save(t target.Target) (target.Target, error) {
	return target.Target{}, nil
}

func TestGet_CheckDefaultNoTargetUrls(t *testing.T) {
	service := target.NewTargetService(&mockTargetRepository{}, config.TargetConfig{})
	ctx := context.Background()

	service.Load()

	urls := []string{
		"/game/service.json", "/game/img.jpeg?t=ret", "/game/service.xml?q=1&w=rrrr",
		"/service/authrootstl.cab", "/work/food/pinrulesstl.cab", "/wg/service_disallowedcertstl.cab",
		"/service/releaselisting_ver2/file.data", "/service/patch_version",
		"/channels/public/rccontent/theme/", "/channels/public/rccontent/theme/?test=test",
	}

	for _, url := range urls {
		tt, err := service.Get(ctx, "example.com", "go-test", url)
		assert.Equal(t, tt.Id, 0)
		assert.ErrorIs(t, err, target.ErrSkipTarget)
	}
}

func TestGet_MatchUserAgent(t *testing.T) {
	service := target.NewTargetService(&mockTargetRepository{}, config.TargetConfig{})
	ctx := context.Background()

	service.Load()

	target, err := service.Get(ctx, "-", "Valve/Steam HTTP Client 1.0", "/game/patch.bin")
	assert.NoError(t, err)
	assert.Equal(t, target.Id, 1)

	target, err = service.Get(ctx, "-", "Steam HTTP Client 1.0", "/game/patch.bin")
	assert.Error(t, err)
}

func TestGet_MatchHostOrSubdomain(t *testing.T) {
	service := target.NewTargetService(&mockTargetRepository{}, config.TargetConfig{})
	ctx := context.Background()

	service.Load()

	// target.Domain == Domain
	target, err := service.Get(ctx, "valve.com", "-", "/game/patch.bin")
	assert.NoError(t, err)
	assert.Equal(t, target.Id, 3)

	// target.Domain == Sub Domain
	target, err = service.Get(ctx, "game.valve.com", "-", "/game/patch.bin")
	assert.NoError(t, err)
	assert.Equal(t, target.Id, 3)

	// target.Domain != Similar Domain
	target, err = service.Get(ctx, "fakevalve.com", "-", "/game/patch.bin")
	assert.NoError(t, err)
	assert.Equal(t, target.Id, 3)
}

func init() {
	log := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	slog.SetDefault(log)
}
