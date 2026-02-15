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
	config *domain.Config
	db     *gorm.DB
	client *client.Client
}

func NewServerRepository(config *domain.Config, db *gorm.DB, cl *client.Client) *ServerRepository {
	return &ServerRepository{
		config: config,
		db:     db,
		client: cl,
	}
}

func (r *ServerRepository) Resolve(ctx context.Context, identifier string, hint *string) (*concrnt.WellKnownConcrnt, error) {
	ctx, span := tracer.Start(ctx, "ServerRepository.Resolve")
	defer span.End()

	if concrnt.IsCSID(identifier) {
		return r.GetAndCacheByCSID(ctx, identifier, hint)
	} else {
		return r.GetAndCacheByFQDN(ctx, identifier)
	}
}

func (r *ServerRepository) GetAndCacheByCSID(ctx context.Context, csid string, hint *string) (*concrnt.WellKnownConcrnt, error) {
	ctx, span := tracer.Start(ctx, "ServerRepository.GetAndCacheByCSID")
	defer span.End()

	var server models.Server
	err := r.db.WithContext(ctx).
		Where("cs_id = ?", csid).
		Take(&server).Error
	if err == nil && server.WellKnown != "" {
		var wkc concrnt.WellKnownConcrnt
		if err := json.Unmarshal([]byte(server.WellKnown), &wkc); err == nil {
			return &wkc, nil
		}
	}

	if hint == nil {
		return nil, domain.ErrNotFound
	}

	wkc, err := r.client.GetServer(ctx, csid, hint)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	serialized, err := json.Marshal(wkc)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	newServer := models.Server{
		ID:        wkc.Domain,
		CSID:      wkc.CSID,
		Layer:     wkc.Layer,
		Tag:       "",
		WellKnown: string(serialized),
	}

	err = r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"cs_id", "layer", "tag", "well_known"}),
	}).Create(&newServer).Error
	if err != nil {
		return nil, err
	}

	return &wkc, nil
}

func (r *ServerRepository) GetAndCacheByFQDN(ctx context.Context, fqdn string) (*concrnt.WellKnownConcrnt, error) {
	ctx, span := tracer.Start(ctx, "ServerRepository.GetAndCacheByFQDN")
	defer span.End()

	var server models.Server
	err := r.db.WithContext(ctx).
		Where("id = ?", fqdn).
		Take(&server).Error
	if err == nil && server.WellKnown != "" {
		var wkc concrnt.WellKnownConcrnt
		if err := json.Unmarshal([]byte(server.WellKnown), &wkc); err == nil {
			return &wkc, nil
		}
	}

	wkc, err := r.client.GetServer(ctx, fqdn, nil)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	serialized, err := json.Marshal(wkc)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	newServer := models.Server{
		ID:        wkc.Domain,
		CSID:      wkc.CSID,
		Layer:     wkc.Layer,
		Tag:       "",
		WellKnown: string(serialized),
	}

	err = r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"cs_id", "layer", "tag", "well_known"}),
	}).Create(&newServer).Error
	if err != nil {
		return nil, err
	}

	return &wkc, nil
}

func (r *ServerRepository) List(ctx context.Context) ([]*concrnt.WellKnownConcrnt, error) {
	var servers []models.Server
	err := r.db.WithContext(ctx).Find(&servers).Error
	if err != nil {
		return nil, err
	}

	result := make([]*concrnt.WellKnownConcrnt, 0, len(servers))
	for _, server := range servers {
		if server.WellKnown == "" {
			continue
		}
		var wkc concrnt.WellKnownConcrnt
		if err := json.Unmarshal([]byte(server.WellKnown), &wkc); err == nil {
			result = append(result, &wkc)
		}
	}

	return result, nil
}
