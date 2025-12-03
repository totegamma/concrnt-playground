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

		var rkid int64 = -1
		if doc.Key != nil {
			var oldRecordKey models.RecordKey
			err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("owner = ? AND key = ?", doc.Owner, doc.Key).
				Take(&oldRecordKey).Error
			if err != nil && err != gorm.ErrRecordNotFound {
				return err
			}

			// RecordKeyを作る

			rk := models.RecordKey{
				Owner:    doc.Author,
				Key:      *doc.Key,
				RecordID: documentID,
			}

			err = tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "owner"}, {Name: "key"}},
				DoUpdates: clause.Assignments(map[string]any{"record_id": documentID}),
			}).Create(&rk).Error
			if err != nil {
				return err
			}

			rkid = rk.ID

			// 古いRecordKeyが指していたRecordを削除する
			if oldRecordKey.RecordID != "" && oldRecordKey.RecordID != documentID {

				if err := tx.Delete(&models.CommitLog{}, "id = ?", oldRecordKey.RecordID).Error; err != nil {
					return err
				}
			}
		}

		// Relationを作る
		if doc.MemberOf != nil {
			for _, parentURI := range *doc.MemberOf {

				collectionRK, err := handleGetRecordKeyIDByURI(ctx, tx, parentURI)
				if err != nil {
					return err
				}

				if rkid == -1 {
					newRecordKey := models.RecordKey{
						Owner:    doc.Author,
						Key:      documentID,
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

			targetRK, err := handleGetRecordKeyIDByURI(ctx, tx, *doc.Associate)
			if err != nil {
				return err
			}

			if rkid == -1 {
				newRecordKey := models.RecordKey{
					Owner:    doc.Author,
					Key:      documentID,
					RecordID: documentID,
				}
				err = tx.Save(&newRecordKey).Error
				if err != nil {
					return err
				}
				rkid = newRecordKey.ID
			}

			association := models.Association{
				TargetID: targetRK,
				ItemID:   rkid,
				Owner:    *doc.Owner,
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

func handleGetRecordKeyIDByURI(ctx context.Context, db *gorm.DB, uri string) (int64, error) {
	owner, key, err := concrnt.ParseCCURI(uri)
	if err != nil {
		return 0, err
	}

	// すでにあればそれを返す
	var recordKey models.RecordKey
	err = db.WithContext(ctx).
		Where("owner = ? AND key = ?", owner, key).
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
		Owner:    owner,
		Key:      record.DocumentID,
		RecordID: record.DocumentID,
	}

	err = db.WithContext(ctx).Create(&newRecordKey).Error
	if err != nil {
		return 0, err
	}

	return newRecordKey.ID, nil
}
