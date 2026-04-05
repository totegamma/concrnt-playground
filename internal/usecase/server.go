package usecase

import (
	"context"

	"github.com/totegamma/concrnt-playground"
	"github.com/totegamma/concrnt-playground/internal/domain"
)

// ServerRepository defines persistence/lookup for remote servers.
type ServerRepository interface {
	Resolve(ctx context.Context, identifier string, hint *string) (*domain.Server, error)
	List(ctx context.Context) ([]*concrnt.WellKnownConcrnt, error)
}

type ServerUsecase struct {
	repo ServerRepository
}

func NewServerUsecase(repo ServerRepository) *ServerUsecase {
	return &ServerUsecase{repo: repo}
}

func (uc *ServerUsecase) Resolve(ctx context.Context, identifier string, hint *string) (*concrnt.WellKnownConcrnt, error) {
	sv, err := uc.repo.Resolve(ctx, identifier, hint)
	if err != nil {
		return nil, err
	}
	return &sv.WellKnown, nil
}

func (uc *ServerUsecase) ResolveWithHint(ctx context.Context, identifier string, hint string) (*concrnt.WellKnownConcrnt, error) {
	sv, err := uc.repo.Resolve(ctx, identifier, &hint)
	if err != nil {
		return nil, err
	}
	return &sv.WellKnown, nil
}

func (uc *ServerUsecase) List(ctx context.Context) ([]*concrnt.WellKnownConcrnt, error) {
	return uc.repo.List(ctx)
}
