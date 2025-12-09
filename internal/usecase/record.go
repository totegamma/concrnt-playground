package usecase

import (
	"context"
	"encoding/json"

	"github.com/totegamma/concrnt-playground"
	"github.com/totegamma/concrnt-playground/schemas"
)

type RecordUsecase struct {
	repo RecordRepository
}

func NewRecordUsecase(repo RecordRepository) *RecordUsecase {
	return &RecordUsecase{repo: repo}
}

func (uc *RecordUsecase) Commit(ctx context.Context, sd concrnt.SignedDocument) error {
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
		return uc.repo.Delete(ctx, string(doc.Value))
	}

	return uc.repo.Create(ctx, sd)
}

func (uc *RecordUsecase) Get(ctx context.Context, uri string) (any, error) {
	return uc.repo.GetValue(ctx, uri)
}
