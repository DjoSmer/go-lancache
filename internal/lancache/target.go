package lancache

import (
	"context"

	"go-lancache/internal/target"
)

type targetService interface {
	Get(cxt context.Context, host string, userAgent string, url string) (target.Target, error)
}
