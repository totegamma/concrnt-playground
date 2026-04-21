package usecase

import (
	"context"
	"strings"

	"github.com/totegamma/concrnt-playground"
	"github.com/totegamma/concrnt-playground/internal/domain"
	"github.com/totegamma/concrnt-playground/schemas"
)

// EntityRepository defines persistence/lookup for entities.
type EntityRepository interface {
	Register(ctx context.Context, sd concrnt.SignedDocument, meta domain.EntityMeta) error
	SaveEntity(ctx context.Context, sd concrnt.SignedDocument, allowLocal bool) (*concrnt.Document[schemas.Entity], error)
	Get(ctx context.Context, ccid string, hint *string) (*domain.Entity, error)
	GetSD(ctx context.Context, ccid string, hint *string) (*concrnt.SignedDocument, error)
	GetDocument(ctx context.Context, ccid string, hint *string) (*concrnt.Document[schemas.Entity], error)
	GetByAlias(ctx context.Context, alias string) (*domain.Entity, error)
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
	return uc.repo.Register(ctx, req.SignedDocument, req.Meta)
}

func (uc *EntityUsecase) SaveEntity(ctx context.Context, sd concrnt.SignedDocument) (*concrnt.SignedDocument, error) {
	_, err := uc.repo.SaveEntity(ctx, sd, true)
	if err != nil {
		return nil, err
	}

	return &sd, nil
}

func (uc *EntityUsecase) Get(ctx context.Context, key string, resolver *string) (*domain.Entity, error) {

	parsed, err := concrnt.ParseCCURI(key)
	if err != nil {
		return nil, err
	}

	id := parsed.Owner

	if strings.HasPrefix(id, "@") {
		return uc.repo.GetByAlias(ctx, id)
	} else {
		return uc.repo.Get(ctx, id, resolver)
	}
}

func (uc *EntityUsecase) GetSD(ctx context.Context, ccid string, resolver *string) (*concrnt.SignedDocument, error) {
	return uc.repo.GetSD(ctx, ccid, resolver)
}

func (uc *EntityUsecase) IsLocal(ctx context.Context, entity domain.Entity) bool {
	return entity.Domain == uc.config.FQDN
}

func (uc *EntityUsecase) IsLocalByCCID(ctx context.Context, ccid string) (bool, error) {
	entity, err := uc.repo.Get(ctx, ccid, nil)
	if err != nil {
		return false, err
	}

	return uc.IsLocal(ctx, *entity), nil
}
