package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/zeebo/xxh3"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/totegamma/concrnt-playground"
	"github.com/totegamma/concrnt-playground/cdid"
	"github.com/totegamma/concrnt-playground/internal/domain"
	"github.com/totegamma/concrnt-playground/internal/infra/database/models"
	"github.com/totegamma/concrnt-playground/internal/utils"
)

type RecordRepository struct {
	db *gorm.DB
}

func NewRecordRepository(db *gorm.DB) *RecordRepository {
	return &RecordRepository{db: db}
}

func (r *RecordRepository) Create(ctx context.Context, sd concrnt.SignedDocument) error {

	var doc concrnt.Document[any]
	err := json.Unmarshal([]byte(sd.Document), &doc)
	if err != nil {
		return err
	}

	hash := concrnt.GetHash([]byte(sd.Document))
	hash10 := [10]byte{}
	copy(hash10[:], hash[:10])
	createAt := doc.CreateAt
	documentID := cdid.New(hash10, createAt).String()

	valueString, err := json.Marshal(doc.Value)
	if err != nil {
		return err
	}

	record := models.Record{
		DocumentID: documentID,
		Value:      string(valueString),
		Author:     doc.Author,
		Schema:     *doc.Schema,
		CDate:      time.Now(),
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {

		proof, err := json.Marshal(sd.Proof)
		if err != nil {
			return err
		}

		commitLog := models.CommitLog{
			ID:       documentID,
			Document: sd.Document,
			Proof:    string(proof),
		}

		if err := tx.Clauses(clause.OnConflict{
			DoNothing: true,
		}).Create(&commitLog).Error; err != nil {
			return err
		}

		var owners []string
		owners = append(owners, doc.Author)
		if doc.Owner != nil && doc.Author != "" {
			owners = append(owners, *doc.Owner)
		}

		for _, owner := range owners {
			err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "commit_log_id"}, {Name: "owner"}},
				DoNothing: true,
			}).Create(&models.CommitOwner{
				CommitLogID: commitLog.ID,
				Owner:       owner,
			}).Error
			if err != nil {
				return err
			}
		}

		if err := tx.Clauses(clause.OnConflict{
			DoNothing: true,
		}).Create(&record).Error; err != nil {
			return err
		}

		var rkid int64 = -1
		if doc.Key != nil {
			var oldRecordKey models.RecordKey
			err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("uri = ?", concrnt.ComposeCCURI(doc.Author, *doc.Key)).
				Take(&oldRecordKey).Error
			if err != nil && err != gorm.ErrRecordNotFound {
				return err
			}

			// RecordKeyを作る

			rk := models.RecordKey{
				URI:      concrnt.ComposeCCURI(doc.Author, *doc.Key),
				RecordID: documentID,
			}

			err = tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "uri"}},
				DoUpdates: clause.Assignments(map[string]any{"record_id": documentID}),
			}).Create(&rk).Error
			if err != nil {
				return err
			}

			rkid = rk.ID

			// 古いRecordKeyが指していたCommitのGCフラグを立て、Recordは消す
			if oldRecordKey.RecordID != "" && oldRecordKey.RecordID != documentID {
				if err := tx.Model(&models.CommitLog{}).
					Where("id = ?", oldRecordKey.RecordID).
					Update("gc_candidate", true).Error; err != nil {
					return err
				}
				if err := tx.Delete(&models.Record{}, "document_id = ?", oldRecordKey.RecordID).Error; err != nil {
					return err
				}
			}
		}

		// Relationを作る
		if doc.MemberOf != nil {
			for _, parentURI := range *doc.MemberOf {

				collectionRK, err := getRecordKeyIDByURI(ctx, tx, parentURI)
				if err != nil {
					return err
				}

				if rkid == -1 {
					newRecordKey := models.RecordKey{
						URI:      concrnt.ComposeCCURI(doc.Author, documentID),
						RecordID: documentID,
					}
					err = tx.Save(&newRecordKey).Error
					if err != nil {
						return err
					}
					rkid = newRecordKey.ID
				}

				relation := models.CollectionMember{
					CollectionID: collectionRK,
					ItemID:       rkid,
				}

				err = tx.Clauses(clause.OnConflict{
					DoNothing: true,
				}).Create(&relation).Error
				if err != nil {
					return err
				}
			}
		}

		// CIP-6 Association
		if doc.Associate != nil {

			targetRK, err := getRecordKeyIDByURI(ctx, tx, *doc.Associate)
			if err != nil {
				return err
			}

			if rkid == -1 {
				newRecordKey := models.RecordKey{
					URI:      concrnt.ComposeCCURI(doc.Author, documentID),
					RecordID: documentID,
				}
				err = tx.Save(&newRecordKey).Error
				if err != nil {
					return err
				}
				rkid = newRecordKey.ID
			}

			uniqueKey := doc.Author + *doc.Associate
			if doc.AssociationVariant != nil {
				uniqueKey += *doc.AssociationVariant
			}
			uniqueHash := xxh3.HashString(uniqueKey)

			association := models.Association{
				TargetID: targetRK,
				ItemID:   rkid,
				Unique:   uniqueHash,
			}
			err = tx.Clauses(clause.OnConflict{
				DoNothing: true,
			}).Create(&association).Error
			if err != nil {
				return err
			}
		}

		return nil
	})
}

func (r *RecordRepository) GetDocument(ctx context.Context, uri string) (*concrnt.Document[any], error) {
	commit, err := getCommitByURI(ctx, r.db, uri)
	if err != nil {
		return nil, err
	}

	var doc concrnt.Document[any]
	err = json.Unmarshal([]byte(commit.Document), &doc)
	if err != nil {
		return nil, err
	}

	return &doc, nil
}

func (r *RecordRepository) GetValue(ctx context.Context, uri string) (any, error) {
	record, err := getRecordByURI(ctx, r.db, uri)
	if err != nil {
		return nil, err
	}

	var value any
	err = json.Unmarshal([]byte(record.Value), &value)
	if err != nil {
		return nil, err
	}

	return value, nil
}

func (r *RecordRepository) Delete(ctx context.Context, uri string) error {

	record, err := getRecordByURI(ctx, r.db, uri)
	if err != nil {
		return err
	}

	id := record.DocumentID

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&models.CommitLog{}, "id = ?", id).Error; err != nil {
			return err
		}
		return nil
	})
}

func getCommitByURI(ctx context.Context, db *gorm.DB, uri string) (*models.CommitLog, error) {

	_, key, err := concrnt.ParseCCURI(uri)
	if err != nil {
		return nil, err
	}

	var commit models.CommitLog
	err = db.WithContext(ctx).
		Where("id = ?", key).
		Take(&commit).Error
	if err == nil {
		return &commit, nil
	}

	var recordKey models.RecordKey
	err = db.WithContext(ctx).Preload("Record.Document").
		Where("uri = ?", uri).
		Take(&recordKey).Error
	if err == nil {
		return &recordKey.Record.Document, nil
	}

	return nil, domain.NotFoundError{Resource: "commit"}

}

func getRecordByURI(ctx context.Context, db *gorm.DB, uri string) (*models.Record, error) {
	_, key, err := concrnt.ParseCCURI(uri)
	if err != nil {
		return nil, err
	}

	var record models.Record
	err = db.WithContext(ctx).
		Where("document_id = ?", key).
		Take(&record).Error
	if err == nil {
		return &record, nil
	}

	var recordKey models.RecordKey
	err = db.WithContext(ctx).Preload("Record").
		Where("uri = ?", uri).
		Take(&recordKey).Error
	if err == nil {
		return &recordKey.Record, nil
	}

	return nil, domain.NotFoundError{Resource: "record"}
}

func getRecordKeyIDByURI(ctx context.Context, db *gorm.DB, uri string) (int64, error) {
	_, key, err := concrnt.ParseCCURI(uri)
	if err != nil {
		return 0, err
	}

	// すでにあればそれを返す
	var recordKey models.RecordKey
	err = db.WithContext(ctx).
		Where("uri = ?", uri).
		Take(&recordKey).Error
	if err == nil {
		return recordKey.ID, nil
	}

	// 参照がCDIDの場合は、新しくRecordKeyを作って返す
	var record models.Record
	err = db.WithContext(ctx).
		Where("document_id = ?", key).
		Take(&record).Error
	if err != nil {
		return 0, err
	}

	newRecordKey := models.RecordKey{
		URI:      uri,
		RecordID: record.DocumentID,
	}

	err = db.WithContext(ctx).Create(&newRecordKey).Error
	if err != nil {
		return 0, err
	}

	return newRecordKey.ID, nil
}

func (r *RecordRepository) GetAssociatedRecords(
	ctx context.Context,
	targetURI, schema, variant, author string,
) ([]concrnt.Document[any], error) {

	targetRKID, err := getRecordKeyIDByURI(ctx, r.db, targetURI)
	if err != nil {
		return nil, err
	}

	var associations []models.Association

	query := r.db.WithContext(ctx).
		Model(&models.Association{}).
		Preload("Item.Record.Document").
		Joins("JOIN record_keys rk ON rk.id = associations.item_id").
		Joins("JOIN records rec ON rec.document_id = rk.record_id").
		Where("associations.target_id = ?", targetRKID)

	if schema != "" {
		query = query.Where("rec.schema = ?", schema)
	}
	if variant != "" {
		query = query.Where("rec.variant = ?", variant)
	}
	if author != "" {
		query = query.Where("rec.author = ?", author)
	}

	if err := query.Find(&associations).Error; err != nil {
		return nil, err
	}

	var documents []concrnt.Document[any]
	for _, assoc := range associations {
		var doc concrnt.Document[any]
		err := json.Unmarshal([]byte(assoc.Item.Record.Document.Document), &doc)
		if err != nil {
			return nil, err
		}
		documents = append(documents, doc)
	}

	return documents, nil
}

func (r *RecordRepository) GetAssociatedRecordCountsBySchema(ctx context.Context, targetURI string) (map[string]int64, error) {

	targetRKID, err := getRecordKeyIDByURI(ctx, r.db, targetURI)
	if err != nil {
		return nil, err
	}

	var counts []struct {
		Schema string
		Count  int64
	}

	err = r.db.WithContext(ctx).
		Model(&models.Association{}).
		Select("rec.schema AS schema, COUNT(*) AS count").
		Joins("JOIN record_keys rk ON rk.id = associations.item_id").
		Joins("JOIN records rec ON rec.document_id = rk.record_id").
		Where("associations.target_id = ?", targetRKID).
		Group("rec.schema").
		Scan(&counts).Error

	if err != nil {
		return nil, err
	}

	result := make(map[string]int64)
	for _, c := range counts {
		result[c.Schema] = c.Count
	}

	return result, nil

}

func (r *RecordRepository) GetAssociatedRecordCountsByVariant(ctx context.Context, targetURI, schema string) (*utils.OrderedKVMap[int64], error) {

	targetRKID, err := getRecordKeyIDByURI(ctx, r.db, targetURI)
	if err != nil {
		return nil, err
	}

	var counts []struct {
		Variant  string
		Count    int64
		MinCDate time.Time
	}

	err = r.db.WithContext(ctx).
		Model(&models.Association{}).
		Select("rec.variant AS variant, COUNT(*) AS count, MIN(rec.c_date) AS min_c_date").
		Joins("JOIN record_keys rk ON rk.id = associations.item_id").
		Joins("JOIN records rec ON rec.document_id = rk.record_id").
		Where("associations.target_id = ? AND rec.schema = ?", targetRKID, schema).
		Group("rec.variant").
		Order("min_c_date ASC").
		Scan(&counts).Error
	if err != nil {
		return nil, err
	}

	result := make(utils.OrderedKVMap[int64])
	for _, c := range counts {
		result[c.Variant] = utils.OrderedKV[int64]{
			Value: c.Count,
			Order: c.MinCDate.UnixNano(),
		}
	}

	return &result, nil
}
