package application

import (
	"context"
	"encoding/json"

	"github.com/totegamma/concrnt-playground"
	"github.com/totegamma/concrnt-playground/internal/infrastructure/database/models"
	"github.com/totegamma/concrnt-playground/internal/infrastructure/repository"
	"github.com/totegamma/concrnt-playground/schemas"
)

type EntityApplication struct {
	repo *repository.EntityRepository
}

func NewEntityApplication(repo *repository.EntityRepository) *EntityApplication {
	return &EntityApplication{repo: repo}
}

func (app *EntityApplication) Register(ctx context.Context, document, signature string, meta models.EntityMeta) error {

	var doc concrnt.Document[schemas.Affiliation]
	err := json.Unmarshal([]byte(document), &document)
	if err != nil {
		return err
	}

	entity := models.Entity{
		ID:                   doc.Author,
		Domain:               doc.Value.Domain,
		AffiliationDocument:  document,
		AffiliationSignature: signature,
	}

	return app.repo.Register(ctx, entity, meta)
}

func (app *EntityApplication) Get(ctx context.Context, ccid string, resolver string) (models.Entity, error) {
	return app.repo.Get(ctx, ccid, resolver)
}
