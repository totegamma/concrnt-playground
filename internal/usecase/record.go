package usecase

import (
	"context"

	"github.com/totegamma/concrnt-playground"
)

// CommitInput is the validated input for committing a record.
type CommitInput struct {
	Document concrnt.Document[any]
	Raw      concrnt.SignedDocument
	Delete   *string
}

type RecordUsecase struct {
	repo RecordRepository
}

func NewRecordUsecase(repo RecordRepository) *RecordUsecase {
	return &RecordUsecase{repo: repo}
}

func (uc *RecordUsecase) Commit(ctx context.Context, input CommitInput) error {
	if input.Delete != nil {
		return uc.repo.Delete(ctx, *input.Delete)
	}
	return uc.repo.Create(ctx, input.Raw)
}

func (uc *RecordUsecase) Get(ctx context.Context, uri string) (any, error) {
	return uc.repo.GetValue(ctx, uri)
}
