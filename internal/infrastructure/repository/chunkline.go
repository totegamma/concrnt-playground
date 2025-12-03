package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/concrnt/chunkline"
	"github.com/totegamma/concrnt-playground/internal/infrastructure/database/models"
)

const (
	defaultChunkSize = 32
)

type ChunklineRepository struct {
	db *gorm.DB
}

func NewChunklineRepository(db *gorm.DB) *ChunklineRepository {
	return &ChunklineRepository{db: db}
}

func (r *ChunklineRepository) LookupLocalItrs(ctx context.Context, uris []string, chunkID int64) (map[string]int64, error) {

	type TimelineRow struct {
		URI      string    `gorm:"column:uri"`
		MaxCDate time.Time `gorm:"column:max_c_date"`
	}

	var res []TimelineRow

	cutoff := time.Unix((chunkID+1)*300, 0) // descending order

	err := r.db.WithContext(ctx).
		Model(&models.CollectionMember{}).
		Joins("JOIN record_keys ON record_keys.id = collection_members.collection_id").
		Select("record_keys.uri AS uri, MAX(collection_members.c_date) AS max_c_date").
		Where("record_keys.uri IN (?) AND collection_members.c_date <= ?", uris, cutoff).
		Group("record_keys.uri").
		Scan(&res).Error

	if err != nil {
		return nil, err
	}

	lookup := make(map[string]int64)
	for _, row := range res {
		lookup[row.URI] = row.MaxCDate.Unix() / 300
	}
	return lookup, nil
}

func (r *ChunklineRepository) LoadLocalBody(ctx context.Context, uri string, chunkID int64) ([]chunkline.BodyItem, error) {

	chunkDate := time.Unix((chunkID+1)*300, 0)
	prevChunkDate := time.Unix((chunkID-1)*300, 0)

	collectionID := int64(0)
	err := r.db.WithContext(ctx).
		Model(&models.RecordKey{}).
		Where("uri = ?", uri).
		Select("id").
		Scan(&collectionID).Error
	if err != nil {
		return nil, err
	}

	var members []models.CollectionMember
	err = r.db.WithContext(ctx).
		Where("collection_id = ?", collectionID).
		Where("c_date <= ?", chunkDate).
		Order("c_date DESC").
		Limit(defaultChunkSize).
		Preload("Item").
		Find(&members).Error
	if err != nil {
		return nil, err
	}

	if len(members) == 0 || members[len(members)-1].CDate.After(prevChunkDate) {
		err = r.db.WithContext(ctx).
			Where("collection_id = ?", collectionID).
			Where("c_date <= ?", chunkDate).
			Where("c_date > ?", prevChunkDate).
			Order("c_date DESC").
			Preload("Item").
			Find(&members).Error
		if err != nil {
			return nil, err
		}
	}

	bodyItems := make([]chunkline.BodyItem, 0, len(members))

	for _, member := range members {
		item := chunkline.BodyItem{
			Timestamp: member.CDate,
			Href:      member.Item.URI,
		}
		bodyItems = append(bodyItems, item)
	}

	return bodyItems, nil
}
