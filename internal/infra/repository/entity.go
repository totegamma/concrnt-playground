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
			Tag:                  entity.Tag,
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

func (r *EntityRepository) Get(ctx context.Context, ccid string, hint *string) (concrnt.Entity, error) {

	var entity models.Entity
	err := r.db.WithContext(ctx).First(&entity, "id = ?", ccid).Error
	if err == nil {
		return concrnt.Entity{
			CCID: entity.ID,
			//Alias:                entity.Alias,
			Domain: entity.Domain,
			//Tag:                  entity.Tag,
			AffiliationDocument:  entity.AffiliationDocument,
			AffiliationSignature: entity.AffiliationSignature,
		}, nil
	}

	if hint == nil || *hint == r.config.FQDN {
		return concrnt.Entity{}, err
	}

	remote, err := r.client.GetEntity(ctx, ccid, hint)
	if err != nil {
		return concrnt.Entity{}, err
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
		return concrnt.Entity{}, err
	}

	return concrnt.Entity{
		CCID:                 newEntity.ID,
		Domain:               newEntity.Domain,
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
