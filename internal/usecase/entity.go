package usecase

import (
	"context"

	"github.com/totegamma/concrnt-playground"
	"github.com/totegamma/concrnt-playground/internal/domain"
	"github.com/totegamma/concrnt-playground/schemas"
)

type EntityRegisterInput struct {
	Document  concrnt.Document[schemas.Affiliation]
	Raw       string
	Signature string
	Meta      domain.EntityMeta
}

type EntityUsecase struct {
	repo EntityRepository
}

func NewEntityUsecase(repo EntityRepository) *EntityUsecase {
	return &EntityUsecase{repo: repo}
}

func (uc *EntityUsecase) Register(ctx context.Context, input EntityRegisterInput) error {
	entity := domain.Entity{
		ID:                   input.Document.Author,
		Domain:               input.Document.Value.Domain,
		AffiliationDocument:  input.Raw,
		AffiliationSignature: input.Signature,
	}

	return uc.repo.Register(ctx, entity, input.Meta)
}

func (uc *EntityUsecase) Get(ctx context.Context, ccid string, resolver string) (domain.Entity, error) {
	return uc.repo.Get(ctx, ccid, resolver)
}
