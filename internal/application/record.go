package application

import (
	"context"
	"encoding/json"

	"github.com/totegamma/concrnt-playground"
	"github.com/totegamma/concrnt-playground/internal/infrastructure/repository"
	"github.com/totegamma/concrnt-playground/schemas"
)

type RecordApplication struct {
	repo *repository.RecordRepository
}

func NewRecordApplication(repo *repository.RecordRepository) *RecordApplication {
	return &RecordApplication{repo: repo}
}

func (app *RecordApplication) Commit(ctx context.Context, sd concrnt.SignedDocument) error {
	var doc concrnt.Document[any]
	err := json.Unmarshal([]byte(sd.Document), &doc)
	if err != nil {
		return err
	}

	if doc.Schema != nil && *doc.Schema == schemas.DeleteURL {

		var doc concrnt.Document[schemas.Delete]
		err := json.Unmarshal([]byte(sd.Document), &doc)
		if err != nil {
			return err
		}

		return app.repo.Delete(ctx, string(doc.Value))
	} else {
		return app.repo.Create(ctx, sd)
	}
}

func (app *RecordApplication) Get(ctx context.Context, uri string) (any, error) {
	return app.repo.GetValue(ctx, uri)
}
