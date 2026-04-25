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
	repo   ServerRepository
	config *domain.Config
}

func NewServerUsecase(
	repo ServerRepository,
	config *domain.Config,
) *ServerUsecase {
	return &ServerUsecase{
		repo:   repo,
		config: config,
	}
}

func (uc *ServerUsecase) Resolve(ctx context.Context, identifier string, hint *string) (*domain.Server, error) {

	if (identifier == uc.config.FQDN) || (identifier == uc.config.CSID) {
		return nil, domain.RedirectError{Location: "https://" + uc.config.FQDN + "/.well-known/concrnt"}
	}

	sv, err := uc.repo.Resolve(ctx, identifier, hint)
	if err != nil {
		return nil, err
	}
	return sv, nil
}

func (uc *ServerUsecase) List(ctx context.Context) ([]*concrnt.WellKnownConcrnt, error) {
	return uc.repo.List(ctx)
}
