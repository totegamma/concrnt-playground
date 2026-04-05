package repository

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/totegamma/concrnt-playground"
	"github.com/totegamma/concrnt-playground/client"
	"github.com/totegamma/concrnt-playground/internal/domain"
	"github.com/totegamma/concrnt-playground/internal/infra/database/models"
)

type EntityRepository struct {
	db     *gorm.DB
	client *client.Client
	config domain.Config
}

func NewEntityRepository(db *gorm.DB, cl *client.Client, config domain.Config) *EntityRepository {
	return &EntityRepository{db: db, client: cl, config: config}
}

func (r *EntityRepository) Register(ctx context.Context, entity domain.Entity, meta domain.EntityMeta) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		modelEntity := models.Entity{
			ID:                   entity.ID,
			Alias:                entity.Alias,
			Domain:               entity.Domain,
			Tag:                  entity.TagString,
			AffiliationDocument:  entity.AffiliationDocument,
			AffiliationSignature: entity.AffiliationSignature,
		}

		modelMeta := models.EntityMeta{
			ID:      meta.ID,
			Inviter: meta.Inviter,
			Info:    meta.Info,
		}

		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{"alias", "domain", "tag", "affiliation_document", "affiliation_signature"}),
		}).Create(&modelEntity).Error; err != nil {
			return err
		}

		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{"inviter", "info"}),
		}).Create(&modelMeta).Error; err != nil {
			return err
		}

		return nil
	})
}

func (r *EntityRepository) Get(ctx context.Context, ccid string, hint *string) (domain.Entity, error) {

	entity, err := gorm.G[models.Entity](r.db).Where("id = ?", ccid).Take(ctx)
	if err == nil {
		return domain.Entity{
			ID:                   entity.ID,
			Alias:                entity.Alias,
			Domain:               entity.Domain,
			TagString:            entity.Tag,
			AffiliationDocument:  entity.AffiliationDocument,
			AffiliationSignature: entity.AffiliationSignature,
		}, nil
	}

	if hint == nil || *hint == r.config.FQDN {
		return domain.Entity{}, domain.NotFoundError{Resource: ccid}
	}

	remote, err := r.client.GetEntity(ctx, ccid, hint)
	if err != nil {
		return domain.Entity{}, err
	}

	newEntity := models.Entity{
		ID:                   remote.CCID,
		Domain:               remote.Domain,
		AffiliationDocument:  remote.AffiliationDocument,
		AffiliationSignature: remote.AffiliationSignature,
	}

	if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"alias", "domain", "tag", "affiliation_document", "affiliation_signature"}),
	}).Create(&newEntity).Error; err != nil {
		return domain.Entity{}, err
	}

	return domain.Entity{
		ID:                   newEntity.ID,
		Alias:                newEntity.Alias,
		Domain:               newEntity.Domain,
		TagString:            newEntity.Tag,
		AffiliationDocument:  newEntity.AffiliationDocument,
		AffiliationSignature: newEntity.AffiliationSignature,
	}, nil
}

func (r *EntityRepository) List(ctx context.Context) ([]concrnt.Entity, error) {
	var entities []models.Entity
	if err := r.db.WithContext(ctx).Find(&entities).Error; err != nil {
		return nil, err
	}

	result := make([]concrnt.Entity, len(entities))
	for i, e := range entities {
		result[i] = concrnt.Entity{
			CCID:                 e.ID,
			Domain:               e.Domain,
			AffiliationDocument:  e.AffiliationDocument,
			AffiliationSignature: e.AffiliationSignature,
		}
	}

	return result, nil
}
