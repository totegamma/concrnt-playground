package repository

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/totegamma/concrnt-playground/client"
	"github.com/totegamma/concrnt-playground/internal/domain"
	"github.com/totegamma/concrnt-playground/internal/infrastructure/database/models"
)

type EntityRepository struct {
	db     *gorm.DB
	client *client.Client
}

func NewEntityRepository(db *gorm.DB, cl *client.Client) *EntityRepository {
	return &EntityRepository{db: db, client: cl}
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

func (r *EntityRepository) Get(ctx context.Context, ccid string, hint string) (domain.Entity, error) {

	var entity models.Entity
	err := r.db.WithContext(ctx).First(&entity, "id = ?", ccid).Error
	if err == nil {
		return domain.Entity{
			ID:                   entity.ID,
			Alias:                entity.Alias,
			Domain:               entity.Domain,
			Tag:                  entity.Tag,
			AffiliationDocument:  entity.AffiliationDocument,
			AffiliationSignature: entity.AffiliationSignature,
		}, nil
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
		Domain:               newEntity.Domain,
		AffiliationDocument:  newEntity.AffiliationDocument,
		AffiliationSignature: newEntity.AffiliationSignature,
	}, nil
}
