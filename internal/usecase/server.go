package usecase

import (
	"context"

	"github.com/totegamma/concrnt-playground"
)

// ServerRepository defines persistence/lookup for remote servers.
type ServerRepository interface {
	Resolve(ctx context.Context, identifier string, hint *string) (*concrnt.WellKnownConcrnt, error)
	List(ctx context.Context) ([]*concrnt.WellKnownConcrnt, error)
}

type ServerUsecase struct {
	repo ServerRepository
}

func NewServerUsecase(repo ServerRepository) *ServerUsecase {
	return &ServerUsecase{repo: repo}
}

func (uc *ServerUsecase) Resolve(ctx context.Context, identifier string, hint *string) (*concrnt.WellKnownConcrnt, error) {
	return uc.repo.Resolve(ctx, identifier, hint)
}

func (uc *ServerUsecase) ResolveWithHint(ctx context.Context, identifier string, hint string) (*concrnt.WellKnownConcrnt, error) {
	return uc.repo.Resolve(ctx, identifier, &hint)
}

func (uc *ServerUsecase) List(ctx context.Context) ([]*concrnt.WellKnownConcrnt, error) {
	return uc.repo.List(ctx)
}
