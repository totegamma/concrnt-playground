package repository

import (
	"context"
	"net/url"
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
	ctx, span := tracer.Start(ctx, "Repository.Chunkline.GetChunklineManifest")
	defer span.End()

	recordKey, err := GetRecordKeyByURI(ctx, r.db, uri)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	ccid, key, err := concrnt.ParseCCURI(uri)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	firstChunk := int64(0)
	firstCollectionMember := models.RecordKey{}
	err = r.db.WithContext(ctx).
		Model(&models.RecordKey{}).
		Joins("JOIN records r ON r.document_id = record_keys.record_id").
		Where("record_keys.parent_id = ?", recordKey.ID).
		Order("r.c_date ASC").
		Limit(1).
		Preload("Record").
		Take(&firstCollectionMember).Error
	if err == nil {
		firstChunk = firstCollectionMember.Record.CDate.Unix() / 600
	}

	safekey := url.PathEscape(key)

	return &chunkline.Manifest{
		Version:    "1.0",
		ChunkSize:  600,
		FirstChunk: firstChunk,
		Descending: &chunkline.Endpoint{
			Iterator: "/chunkline/" + ccid + "/" + safekey + "/{chunk}/itr",
			Body:     "/chunkline/" + ccid + "/" + safekey + "/{chunk}/body",
		},
		// Metadata: recordKey.Record.Value,
	}, nil
}

func (r *ChunklineRepository) LookupLocalItrs(ctx context.Context, uris []string, chunkID int64) (map[string]int64, error) {
	ctx, span := tracer.Start(ctx, "Repository.Chunkline.LookupLocalItrs")
	defer span.End()

	type TimelineRow struct {
		URI      string    `gorm:"column:uri"`
		MaxCDate time.Time `gorm:"column:max_c_date"`
	}

	var res []TimelineRow

	cutoff := time.Unix((chunkID+1)*600, 0) // descending order

	err := r.db.WithContext(ctx).
		Table("record_keys AS parent").
		Joins("JOIN record_keys AS child ON child.parent_id = parent.id").
		Joins("JOIN records r ON r.document_id = child.record_id").
		Select("parent.uri AS uri, MAX(r.c_date) AS max_c_date").
		Where("parent.uri IN ? AND r.c_date <= ?", uris, cutoff).
		Group("parent.uri").
		Scan(&res).Error

	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	lookup := make(map[string]int64)
	for _, row := range res {
		lookup[row.URI] = row.MaxCDate.Unix() / 600
	}
	return lookup, nil
}

func (r *ChunklineRepository) LoadLocalBody(ctx context.Context, uri string, chunkID int64) ([]chunkline.BodyItem, error) {
	ctx, span := tracer.Start(ctx, "Repository.Chunkline.LoadLocalBody")
	defer span.End()

	chunkDate := time.Unix((chunkID+1)*600, 0)
	prevChunkDate := time.Unix((chunkID-1)*600, 0)

	parentRecordKey, err := GetRecordKeyByURI(ctx, r.db, uri)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	var members []models.RecordKey
	err = r.db.WithContext(ctx).
		Joins("JOIN records r ON r.document_id = record_keys.record_id").
		Where("parent_id = ?", parentRecordKey.ID).
		Where("r.c_date <= ?", chunkDate).
		Order("r.c_date DESC").
		Limit(defaultChunkSize).
		Preload("Record").
		Preload("Record.Document").
		Find(&members).Error
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	if len(members) == 0 || members[len(members)-1].Record.CDate.After(prevChunkDate) {
		err = r.db.WithContext(ctx).
			Joins("JOIN records r ON r.document_id = record_keys.record_id").
			Where("parent_id = ?", parentRecordKey.ID).
			Where("r.c_date <= ?", chunkDate).
			Where("r.c_date > ?", prevChunkDate).
			Order("r.c_date DESC").
			Preload("Record").
			Preload("Record.Document").
			Find(&members).Error
		if err != nil {
			span.RecordError(err)
			return nil, err
		}
	}

	bodyItems := make([]chunkline.BodyItem, 0, len(members))

	for _, member := range members {

		href := member.URI
		contentType := "application/concrnt.document+json"
		if member.Record.Schema == schemas.ReferenceURL {
			var itemURLValue concrnt.Document[schemas.Reference]
			err = json.Unmarshal([]byte(member.Record.Document.Document), &itemURLValue)
			if err == nil {
				if itemURLValue.Value.Href != "" {
					href = itemURLValue.Value.Href
				}
				if itemURLValue.Value.ContentType != "" {
					contentType = itemURLValue.Value.ContentType
				}
			} else {
				span.RecordError(err)
			}
		}

		item := chunkline.BodyItem{
			Timestamp:   member.Record.CDate,
			Href:        href,
			ContentType: contentType,
		}
		bodyItems = append(bodyItems, item)
	}

	return bodyItems, nil
}
