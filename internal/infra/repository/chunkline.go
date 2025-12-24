package repository

import (
	"context"
	"time"

	"encoding/json"
	"gorm.io/gorm"

	"github.com/concrnt/chunkline"
	"github.com/totegamma/concrnt-playground"
	"github.com/totegamma/concrnt-playground/internal/infra/database/models"
	"github.com/totegamma/concrnt-playground/schemas"
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

func (r *ChunklineRepository) GetChunklineManifest(ctx context.Context, uri string) (*chunkline.Manifest, error) {

	record, err := getRecordByURI(ctx, r.db, uri)
	if err != nil {
		return nil, err
	}

	ccid, key, err := concrnt.ParseCCURI(uri)
	if err != nil {
		return nil, err
	}

	firstChunk := int64(0)
	firstCollectionMember := models.PrefixGroup{}
	err = r.db.WithContext(ctx).
		Model(&models.PrefixGroup{}).
		Joins("JOIN record_keys rk ON rk.id = prefix_groups.collection_id").
		Joins("JOIN records r ON r.document_id = prefix_groups.item_id").
		Where("rk.uri = ?", uri).
		Order("r.c_date ASC").
		Limit(1).
		// PreloadはこのままでもOK（Itemは別クエリで埋まる）
		Preload("Item").
		Take(&firstCollectionMember).Error
	if err == nil {
		firstChunk = firstCollectionMember.Item.CDate.Unix() / 600
	}

	return &chunkline.Manifest{
		Version:    "1.0",
		ChunkSize:  600,
		FirstChunk: firstChunk,
		Descending: &chunkline.Endpoint{
			Iterator: "/chunkline/" + ccid + "/" + key + "/{chunk}/itr",
			Body:     "/chunkline/" + ccid + "/" + key + "/{chunk}/body",
		},
		Metadata: record.Value,
	}, nil
}

func (r *ChunklineRepository) LookupLocalItrs(ctx context.Context, uris []string, chunkID int64) (map[string]int64, error) {

	type TimelineRow struct {
		URI      string    `gorm:"column:uri"`
		MaxCDate time.Time `gorm:"column:max_c_date"`
	}

	var res []TimelineRow

	cutoff := time.Unix((chunkID+1)*600, 0) // descending order

	err := r.db.WithContext(ctx).
		Model(&models.PrefixGroup{}).
		Joins("JOIN record_keys rk ON rk.id = prefix_groups.collection_id").
		Joins("JOIN records r ON r.document_id = prefix_groups.item_id").
		Select("rk.uri AS uri, MAX(r.c_date) AS max_c_date").
		Where("rk.uri IN ? AND r.c_date <= ?", uris, cutoff).
		Group("rk.uri").
		Scan(&res).Error

	if err != nil {
		return nil, err
	}

	lookup := make(map[string]int64)
	for _, row := range res {
		lookup[row.URI] = row.MaxCDate.Unix() / 600
	}
	return lookup, nil
}

func (r *ChunklineRepository) LoadLocalBody(ctx context.Context, uri string, chunkID int64) ([]chunkline.BodyItem, error) {

	chunkDate := time.Unix((chunkID+1)*600, 0)
	prevChunkDate := time.Unix((chunkID-1)*600, 0)

	collectionID := int64(0)
	err := r.db.WithContext(ctx).
		Model(&models.RecordKey{}).
		Where("uri = ?", uri).
		Select("id").
		Scan(&collectionID).Error
	if err != nil {
		return nil, err
	}

	var members []models.PrefixGroup
	err = r.db.WithContext(ctx).
		Joins("JOIN records r ON r.document_id = prefix_groups.item_id").
		Where("collection_id = ?", collectionID).
		Where("r.c_date <= ?", chunkDate).
		Order("r.c_date DESC").
		Limit(defaultChunkSize).
		Preload("Item").
		Find(&members).Error
	if err != nil {
		return nil, err
	}

	if len(members) == 0 || members[len(members)-1].Item.CDate.After(prevChunkDate) {
		err = r.db.WithContext(ctx).
			Joins("JOIN records r ON r.document_id = prefix_groups.item_id").
			Where("collection_id = ?", collectionID).
			Where("r.c_date <= ?", chunkDate).
			Where("r.c_date > ?", prevChunkDate).
			Order("r.c_date DESC").
			Preload("Item").
			Find(&members).Error
		if err != nil {
			return nil, err
		}
	}

	bodyItems := make([]chunkline.BodyItem, 0, len(members))

	for _, member := range members {

		href := concrnt.ComposeCCURI(member.Item.Owner, member.Item.DocumentID)
		contentType := "application/concrnt.document+json"
		if member.Item.Schema == schemas.ReferenceURL {
			var itemURLValue schemas.Reference
			err = json.Unmarshal([]byte(member.Item.Value), &itemURLValue)
			if err == nil {
				if itemURLValue.Href != "" {
					href = itemURLValue.Href
				}
				if itemURLValue.ContentType != "" {
					contentType = itemURLValue.ContentType
				}
			}
		}

		item := chunkline.BodyItem{
			Timestamp:   member.Item.CDate,
			Href:        href,
			ContentType: contentType,
		}
		bodyItems = append(bodyItems, item)
	}

	return bodyItems, nil
}
