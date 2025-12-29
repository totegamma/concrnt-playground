package usecase

import (
	"context"
	"encoding/json"
	"time"

	"github.com/totegamma/concrnt-playground"
	"github.com/totegamma/concrnt-playground/internal/utils"
	"github.com/totegamma/concrnt-playground/schemas"
)

// RecordRepository defines storage operations for records/commits.
type RecordRepository interface {
	CreateRecord(ctx context.Context, sd concrnt.SignedDocument) error
	CreateAssociation(ctx context.Context, sd concrnt.SignedDocument) error
	Delete(ctx context.Context, sd concrnt.SignedDocument) error

	GetDocument(ctx context.Context, uri string) (*concrnt.Document[any], error)
	GetSignedDocument(ctx context.Context, uri string) (*concrnt.SignedDocument, error)

	GetAssociatedRecords(ctx context.Context, targetURI, schema, variant, author string) ([]concrnt.Document[any], error)
	GetAssociatedRecordCountsBySchema(ctx context.Context, targetURI string) (map[string]int64, error)
	GetAssociatedRecordCountsByVariant(ctx context.Context, targetURI, schema string) (*utils.OrderedKVMap[int64], error)
	Query(ctx context.Context, prefix, schema string, since, until *time.Time, limit int, order string) ([]concrnt.Document[any], error)
}

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

	switch doc.Schema {
	case schemas.DeleteURL:
		return uc.repo.Delete(ctx, sd)
	default:
		if doc.Associate != nil {
			return uc.repo.CreateAssociation(ctx, sd)
		} else {
			return uc.repo.CreateRecord(ctx, sd)
		}
	}
}

func (uc *RecordUsecase) Get(ctx context.Context, uri string) (*concrnt.Document[any], error) {
	return uc.repo.GetDocument(ctx, uri)
}

func (uc *RecordUsecase) GetSigned(ctx context.Context, uri string) (*concrnt.SignedDocument, error) {
	return uc.repo.GetSignedDocument(ctx, uri)
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

func (uc *RecordUsecase) Query(
	ctx context.Context,
	prefix, schema string,
	since, until *time.Time,
	limit int,
	order string,
) ([]concrnt.Document[any], error) {
	return uc.repo.Query(ctx, prefix, schema, since, until, limit, order)
}
