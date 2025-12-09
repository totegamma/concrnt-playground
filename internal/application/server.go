package application

import (
	"context"

	"github.com/totegamma/concrnt-playground"
	"github.com/totegamma/concrnt-playground/internal/infrastructure/repository"
)

type ServerApplication struct {
	repo *repository.ServerRepository
}

func NewServerApplication(repo *repository.ServerRepository) *ServerApplication {
	return &ServerApplication{repo: repo}
}

func (app *ServerApplication) Resolve(ctx context.Context, identifier, hint string) (concrnt.WellKnownConcrnt, error) {
	srv, err := app.repo.Resolve(ctx, identifier, hint)
	if err != nil {
		return concrnt.WellKnownConcrnt{}, err
	}
	return srv.WellKnown, nil
}
