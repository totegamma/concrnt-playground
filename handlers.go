package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/concrnt/concrnt/cdid"
	"github.com/concrnt/concrnt/core"
)

func handleInsert(ctx context.Context, db *gorm.DB, sd SignedDocument) error {

	var doc Document[any]
	err := json.Unmarshal([]byte(sd.Document), &doc)
	if err != nil {
		return err
	}

	hash := core.GetHash([]byte(sd.Document))
	hash10 := [10]byte{}
	copy(hash10[:], hash[:10])
	signedAt := doc.SignedAt
	documentID := cdid.New(hash10, signedAt).String()

	valueString, err := json.Marshal(doc.Value)
	if err != nil {
		return err
	}

	record := Record{
		DocumentID: documentID,
		Value:      string(valueString),
		Author:     doc.Author,
		Schema:     *doc.Schema,
		CDate:      time.Now(),
	}

	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {

		proof, err := json.Marshal(sd.Proof)
		if err != nil {
			return err
		}

		commitLog := CommitLog{
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
			}).Create(&CommitOwner{
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

		var oldRecordKey RecordKey
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
		}).Create(&RecordKey{
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

				relation := CollectionMember{
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
			association := Association{
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

			if err := tx.Model(&CollectionMember{}).
				Where("parent_id = ?", oldID).
				Update("parent_id", newID).Error; err != nil {
				return err
			}

			if err := tx.Model(&CollectionMember{}).
				Where("child_id = ?", oldID).
				Update("child_id", newID).Error; err != nil {
				return err
			}

			if err := tx.Model(&Association{}).
				Where("target_id = ?", oldID).
				Update("target_id", newID).Error; err != nil {
				return err
			}

			if err := tx.Model(&Association{}).
				Where("item_id = ?", oldID).
				Update("item_id", newID).Error; err != nil {
				return err
			}

			if err := tx.Delete(&CommitLog{}, "id = ?", oldRecordKey.RecordID).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

func handleDelete(ctx context.Context, db *gorm.DB, sd SignedDocument) error {

	var doc Document[SchemaDeleteType]
	err := json.Unmarshal([]byte(sd.Document), &doc)
	if err != nil {
		return err
	}

	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&CommitLog{}, "id = ?", doc.Value).Error; err != nil {
			return err
		}
		return nil
	})

}

func handleGetChildsByID(ctx context.Context, db *gorm.DB, parentID string) ([]Record, error) {
	var records []Record
	err := db.WithContext(ctx).
		Model(&Record{}).
		Joins("JOIN record_relations rr ON rr.child_id = records.document_id").
		Where("rr.parent_id = ?", parentID).
		Order("records.c_date").
		Find(&records).Error
	if err != nil {
		return nil, err
	}
	return records, nil
}

func handleGetChilds(ctx context.Context, db *gorm.DB, parentURI string) ([]Record, error) {
	// 1. 親 Record を URI から解決
	parentRecord, err := handleGetRecordByURI(ctx, db, parentURI)
	if err != nil {
		return nil, err
	}

	// 2. 親のDocumentIDから子を取る
	return handleGetChildsByID(ctx, db, parentRecord.DocumentID)
}

func handleGetRecordByURI(ctx context.Context, db *gorm.DB, uri string) (*Record, error) {
	owner, key, err := ParseCCURI(uri)
	if err != nil {
		return nil, err
	}

	var record Record
	err = db.WithContext(ctx).
		Where("document_id = ?", key).
		Take(&record).Error
	if err == nil {
		return &record, nil
	}

	var recordKey RecordKey
	err = db.WithContext(ctx).Preload("Record").
		Where("owner = ? AND key = ?", owner, key).
		Take(&recordKey).Error
	if err == nil {
		return &recordKey.Record, nil
	}

	return nil, fmt.Errorf("record not found")
}

func handleGetRecordByKey(ctx context.Context, db *gorm.DB, owner string, key string) (*Record, error) {
	var recordKey RecordKey
	err := db.WithContext(ctx).Preload("Record").
		Where("owner = ? AND key = ?", owner, key).
		Take(&recordKey).Error
	if err != nil {
		return nil, err
	}
	return &recordKey.Record, nil
}

func handleGetRecordByID(ctx context.Context, db *gorm.DB, documentID string) (*Record, error) {
	var record Record
	err := db.WithContext(ctx).
		Where("document_id = ?", documentID).
		Take(&record).Error
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func HandleCommit(ctx context.Context, db *gorm.DB, sd SignedDocument) error {

	var doc Document[any]
	err := json.Unmarshal([]byte(sd.Document), &doc)
	if err != nil {
		return err
	}

	if doc.Schema != nil && *doc.Schema == "concrnt/schema/delete" {
		return handleDelete(ctx, db, sd)
	} else {
		return handleInsert(ctx, db, sd)
	}

}
