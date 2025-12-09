package usecase

import (
	"context"
	"encoding/json"

	"github.com/totegamma/concrnt-playground"
	"github.com/totegamma/concrnt-playground/internal/domain"
	"github.com/totegamma/concrnt-playground/schemas"
)

type EntityUsecase struct {
	repo EntityRepository
}

func NewEntityUsecase(repo EntityRepository) *EntityUsecase {
	return &EntityUsecase{repo: repo}
}

func (uc *EntityUsecase) Register(ctx context.Context, document, signature string, meta domain.EntityMeta) error {

	var doc concrnt.Document[schemas.Affiliation]
	err := json.Unmarshal([]byte(document), &doc)
	if err != nil {
		return err
	}

	entity := domain.Entity{
		ID:                   doc.Author,
		Domain:               doc.Value.Domain,
		AffiliationDocument:  document,
		AffiliationSignature: signature,
	}

	return uc.repo.Register(ctx, entity, meta)
}

func (uc *EntityUsecase) Get(ctx context.Context, ccid string, resolver string) (domain.Entity, error) {
	return uc.repo.Get(ctx, ccid, resolver)
}
