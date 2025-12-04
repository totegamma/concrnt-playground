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

func (app *ServerApplication) Resolve(ctx context.Context, identifier string) (concrnt.WellKnownConcrnt, error) {
	return app.repo.Get(ctx, identifier)
}
