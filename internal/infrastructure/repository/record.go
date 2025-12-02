package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/totegamma/concrnt-playground"
	"github.com/totegamma/concrnt-playground/cdid"
	"github.com/totegamma/concrnt-playground/internal/infrastructure/database/models"
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

		// Keyが空文字列の場合はRecordKeyを作らない
		if doc.Key == nil {
			return nil
		}

		var oldRecordKey models.RecordKey
		err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("owner = ? AND key = ?", doc.Owner, doc.Key).
			Take(&oldRecordKey).Error
		if err != nil && err != gorm.ErrRecordNotFound {
			return err
		}

		// RecordKeyを作る
		err = tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "owner"}, {Name: "key"}},
			DoUpdates: clause.Assignments(map[string]any{"record_id": documentID}),
		}).Create(&models.RecordKey{
			Owner:    *doc.Owner,
			Key:      *doc.Key,
			RecordID: documentID,
		}).Error
		if err != nil {
			return err
		}

		// Relationを作る
		if doc.MemberOf != nil {
			for _, parentURI := range *doc.MemberOf {
				parentRecord, err := handleGetRecordByURI(ctx, tx, parentURI)
				if err != nil {
					return err
				}

				relation := models.CollectionMember{
					CollectionID: parentRecord.DocumentID,
					ItemID:       documentID,
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
			targetRecord, err := handleGetRecordByURI(ctx, tx, *doc.Associate)
			if err != nil {
				return err
			}
			association := models.Association{
				TargetID: targetRecord.DocumentID,
				ItemID:   documentID,
				Owner:    *doc.Owner,
			}
			err = tx.Clauses(clause.OnConflict{
				DoNothing: true,
			}).Create(&association).Error
			if err != nil {
				return err
			}
		}

		// 古いRecordKeyが指していたRecordを削除する
		if oldRecordKey.RecordID != "" && oldRecordKey.RecordID != documentID {

			oldID := oldRecordKey.RecordID
			newID := documentID

			if err := tx.Model(&models.CollectionMember{}).
				Where("parent_id = ?", oldID).
				Update("parent_id", newID).Error; err != nil {
				return err
			}

			if err := tx.Model(&models.CollectionMember{}).
				Where("child_id = ?", oldID).
				Update("child_id", newID).Error; err != nil {
				return err
			}

			if err := tx.Model(&models.Association{}).
				Where("target_id = ?", oldID).
				Update("target_id", newID).Error; err != nil {
				return err
			}

			if err := tx.Model(&models.Association{}).
				Where("item_id = ?", oldID).
				Update("item_id", newID).Error; err != nil {
				return err
			}

			if err := tx.Delete(&models.CommitLog{}, "id = ?", oldRecordKey.RecordID).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *RecordRepository) GetValue(ctx context.Context, uri string) (any, error) {
	record, err := handleGetRecordByURI(ctx, r.db, uri)
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

	record, err := handleGetRecordByURI(ctx, r.db, uri)
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

func handleGetRecordByURI(ctx context.Context, db *gorm.DB, uri string) (*models.Record, error) {
	owner, key, err := concrnt.ParseCCURI(uri)
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
		Where("owner = ? AND key = ?", owner, key).
		Take(&recordKey).Error
	if err == nil {
		return &recordKey.Record, nil
	}

	return nil, fmt.Errorf("record not found")
}
