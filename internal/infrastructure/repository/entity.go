package repository

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/totegamma/concrnt-playground/client"
	"github.com/totegamma/concrnt-playground/internal/infrastructure/database/models"
)

type EntityRepository struct {
	db     *gorm.DB
	client *client.Client
}

func NewEntityRepository(db *gorm.DB, cl *client.Client) *EntityRepository {
	return &EntityRepository{db: db, client: cl}
}

func (r *EntityRepository) Register(ctx context.Context, entity models.Entity, meta models.EntityMeta) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{"alias", "domain", "tag", "affiliation_document", "affiliation_signature"}),
		}).Create(&entity).Error; err != nil {
			return err
		}

		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{"inviter", "info"}),
		}).Create(&meta).Error; err != nil {
			return err
		}

		return nil
	})
}

func (r *EntityRepository) Get(ctx context.Context, ccid string, hint string) (models.Entity, error) {

	var entity models.Entity
	err := r.db.WithContext(ctx).First(&entity, "id = ?", ccid).Error
	if err == nil {
		return entity, nil
	}

	remote, err := r.client.GetEntityWithResolver(ctx, hint, ccid)
	if err != nil {
		return models.Entity{}, err
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
		return models.Entity{}, err
	}

	return newEntity, nil
}
