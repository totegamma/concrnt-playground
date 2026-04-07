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
	Get(ctx context.Context, ccid string, resolver *string) (domain.Entity, error)
	List(ctx context.Context) ([]concrnt.Entity, error)
}

type EntityUsecase struct {
	repo   EntityRepository
	config *domain.Config
}

func NewEntityUsecase(
	repo EntityRepository,
	config *domain.Config,
) *EntityUsecase {
	return &EntityUsecase{
		repo:   repo,
		config: config,
	}
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

func (uc *EntityUsecase) GetProper(ctx context.Context, ccid string, resolver *string) (concrnt.Entity, error) {
	domainEntity, err := uc.repo.Get(ctx, ccid, resolver)
	if err != nil {
		return concrnt.Entity{}, err
	}

	return concrnt.Entity{
		CCID:                 domainEntity.ID,
		Domain:               domainEntity.Domain,
		AffiliationDocument:  domainEntity.AffiliationDocument,
		AffiliationSignature: domainEntity.AffiliationSignature,
	}, nil
}

func (uc *EntityUsecase) Get(ctx context.Context, ccid string, resolver *string) (domain.Entity, error) {
	return uc.repo.Get(ctx, ccid, resolver)
}

func (uc *EntityUsecase) List(ctx context.Context) ([]concrnt.Entity, error) {
	return uc.repo.List(ctx)
}

func (uc *EntityUsecase) IsLocal(ctx context.Context, entity domain.Entity) bool {
	return entity.Domain == uc.config.FQDN
}

func (uc *EntityUsecase) IsLocalByCCID(ctx context.Context, ccid string) (bool, error) {
	entity, err := uc.repo.Get(ctx, ccid, nil)
	if err != nil {
		return false, err
	}

	return uc.IsLocal(ctx, entity), nil
}
