package repository

import (
	"context"
	"encoding/json"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/totegamma/concrnt-playground"
	"github.com/totegamma/concrnt-playground/client"
	"github.com/totegamma/concrnt-playground/internal/domain"
	"github.com/totegamma/concrnt-playground/internal/infra/database/models"
)

type ServerRepository struct {
	db     *gorm.DB
	client *client.Client
}

func NewServerRepository(db *gorm.DB, cl *client.Client) *ServerRepository {
	return &ServerRepository{db: db, client: cl}
}

func (r *ServerRepository) Resolve(ctx context.Context, identifier, hint string) (domain.Server, error) {

	var server models.Server
	err := r.db.WithContext(ctx).
		Where("id = ? OR cs_id = ?", identifier, identifier).
		Take(&server).Error
	if err == nil && server.WellKnown != "" {
		var wkc concrnt.WellKnownConcrnt
		if err := json.Unmarshal([]byte(server.WellKnown), &wkc); err == nil {
			return domain.Server{
				Domain:    server.ID,
				CSID:      server.CSID,
				Layer:     server.Layer,
				Version:   server.Tag,
				WellKnown: wkc,
			}, nil
		}
	}

	if hint == "" {
		hint = identifier
	}

	wkc, err := r.client.GetServer(ctx, identifier, hint)
	if err != nil {
		return domain.Server{}, err
	}

	serialized, err := json.Marshal(wkc)
	if err != nil {
		return domain.Server{}, err
	}

	newServer := models.Server{
		ID:        wkc.Domain,
		CSID:      wkc.CSID,
		Layer:     wkc.Layer,
		Tag:       wkc.Version,
		WellKnown: string(serialized),
	}

	err = r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"cs_id", "layer", "tag", "well_known"}),
	}).Create(&newServer).Error
	if err != nil {
		return domain.Server{}, err
	}

	return domain.Server{
		Domain:    newServer.ID,
		CSID:      newServer.CSID,
		Layer:     newServer.Layer,
		Version:   newServer.Tag,
		WellKnown: wkc,
	}, nil
}
