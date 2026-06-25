package target

import (
	"context"
	"errors"
)

var ErrTargetNotFound = errors.New("target not found")

type TargetRepository interface {
	FindById(ctx context.Context, id int) (Target, error)
	GetAll(ctx context.Context) ([]Target, error)
	Save(target Target) (Target, error)
}
