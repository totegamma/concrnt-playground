package usecase

import (
	"context"

	"github.com/totegamma/concrnt-playground"
	"github.com/totegamma/concrnt-playground/internal/utils"
)

// CommitInput is the validated input for committing a record.
type CommitInput struct {
	Document concrnt.Document[any]
	Raw      concrnt.SignedDocument
	Delete   *string
}

// RecordRepository defines storage operations for records/commits.
type RecordRepository interface {
	Create(ctx context.Context, sd concrnt.SignedDocument) error
	GetValue(ctx context.Context, uri string) (any, error)
	GetDocument(ctx context.Context, uri string) (*concrnt.Document[any], error)
	Delete(ctx context.Context, uri string) error

	GetAssociatedRecords(ctx context.Context, targetURI, schema, variant, author string) ([]concrnt.Document[any], error)
	GetAssociatedRecordCountsBySchema(ctx context.Context, targetURI string) (map[string]int64, error)
	GetAssociatedRecordCountsByVariant(ctx context.Context, targetURI, schema string) (*utils.OrderedKVMap[int64], error)
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

func (uc *RecordUsecase) Get(ctx context.Context, uri string) (*concrnt.Document[any], error) {
	return uc.repo.GetDocument(ctx, uri)
}

func (uc *RecordUsecase) GetValue(ctx context.Context, uri string) (any, error) {
	return uc.repo.GetValue(ctx, uri)
}

func (uc *RecordUsecase) GetAssociatedRecords(ctx context.Context, targetURI, schema, variant, author string) ([]concrnt.Document[any], error) {
	return uc.repo.GetAssociatedRecords(ctx, targetURI, schema, variant, author)
}

func (uc *RecordUsecase) GetAssociatedRecordCountsBySchema(ctx context.Context, targetURI string) (map[string]int64, error) {
	return uc.repo.GetAssociatedRecordCountsBySchema(ctx, targetURI)
}

func (uc *RecordUsecase) GetAssociatedRecordCountsByVariant(ctx context.Context, targetURI, schema string) (*utils.OrderedKVMap[int64], error) {
	return uc.repo.GetAssociatedRecordCountsByVariant(ctx, targetURI, schema)
}
