package usecase

import (
	"context"

	"github.com/totegamma/concrnt-playground/internal/domain"
)

// ServerRepository defines persistence/lookup for remote servers.
type ServerRepository interface {
	Resolve(ctx context.Context, identifier, hint string) (domain.Server, error)
}

type ServerUsecase struct {
	repo ServerRepository
}

func NewServerUsecase(repo ServerRepository) *ServerUsecase {
	return &ServerUsecase{repo: repo}
}

func (uc *ServerUsecase) Resolve(ctx context.Context, identifier, hint string) (domain.Server, error) {
	return uc.repo.Resolve(ctx, identifier, hint)
}
