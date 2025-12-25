package usecase

import (
	"context"
	"encoding/json"

	"github.com/totegamma/concrnt-playground"
	"github.com/totegamma/concrnt-playground/internal/domain"
	"github.com/totegamma/concrnt-playground/schemas"
)

// EntityRepository defines persistence/lookup for entities.
type EntityRepository interface {
	Register(ctx context.Context, entity domain.Entity, meta domain.EntityMeta) error
	Get(ctx context.Context, ccid string, resolver string) (domain.Entity, error)
}

type EntityUsecase struct {
	repo EntityRepository
}

func NewEntityUsecase(repo EntityRepository) *EntityUsecase {
	return &EntityUsecase{repo: repo}
}

func (uc *EntityUsecase) Register(ctx context.Context, req concrnt.RegisterRequest[domain.EntityMeta]) error {

	var doc concrnt.Document[schemas.Affiliation]
	if err := json.Unmarshal([]byte(req.AffiliationDocument), &doc); err != nil {
		return err
	}

	entity := domain.Entity{
		ID:                   doc.Author,
		Domain:               doc.Value.Domain,
		AffiliationDocument:  req.AffiliationDocument,
		AffiliationSignature: req.AffiliationSignature,
	}

	return uc.repo.Register(ctx, entity, req.Meta)
}

func (uc *EntityUsecase) Get(ctx context.Context, ccid string, resolver string) (domain.Entity, error) {
	return uc.repo.Get(ctx, ccid, resolver)
}
