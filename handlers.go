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

func handleInsert(ctx context.Context, db *gorm.DB, com Commit, doc Document) error {

	hash := core.GetHash([]byte(com.Document))
	hash10 := [10]byte{}
	copy(hash10[:], hash[:10])
	signedAt := doc.SignedAt
	documentID := cdid.New(hash10, signedAt).String()

	record := Record{
		DocumentID: documentID,
		Value:      doc.Value,
		Owner:      doc.Owner,
		Signer:     doc.Signer,
		Schema:     doc.Schema,
		CDate:      time.Now(),
	}

	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {

		commitLog := CommitLog{
			ID:        documentID,
			Document:  com.Document,
			Signature: com.Signature,
		}

		if err := tx.Clauses(clause.OnConflict{
			DoNothing: true,
		}).Create(&commitLog).Error; err != nil {
			return err
		}

		var owners []string
		if doc.Owner != "" {
			owners = append(owners, doc.Owner)
		}
		if doc.Signer != "" && doc.Signer != doc.Owner {
			owners = append(owners, doc.Signer)
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
		if doc.Key == "" {
			return nil
		}

		var oldRecordKey RecordKey
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
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
			Owner:    doc.Owner,
			Key:      doc.Key,
			RecordID: documentID,
		}).Error
		if err != nil {
			return err
		}

		// Relationを作る
		for _, parentURI := range doc.Referenced {
			parentRecord, err := handleGetRecordByURI(ctx, tx, parentURI)
			if err != nil {
				return err
			}

			relation := RecordRelation{
				ParentID: parentRecord.DocumentID,
				ChildID:  documentID,
			}

			err = tx.Clauses(clause.OnConflict{
				DoNothing: true,
			}).Create(&relation).Error
			if err != nil {
				return err
			}
		}

		// 古いRecordKeyが指していたRecordを削除する
		if oldRecordKey.RecordID != "" && oldRecordKey.RecordID != documentID {

			oldID := oldRecordKey.RecordID
			newID := documentID

			if err := tx.Model(&RecordRelation{}).
				Where("parent_id = ?", oldID).
				Update("parent_id", newID).Error; err != nil {
				return err
			}

			if err := tx.Model(&RecordRelation{}).
				Where("child_id = ?", oldID).
				Update("child_id", newID).Error; err != nil {
				return err
			}

			if err := tx.Delete(&CommitLog{}, "id = ?", oldRecordKey.RecordID).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

func handleDelete(ctx context.Context, db *gorm.DB, doc Document) error {

	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&CommitLog{}, "id = ?", doc.Reference).Error; err != nil {
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

func HandleCommit(ctx context.Context, db *gorm.DB, commit Commit) error {

	var doc Document
	err := json.Unmarshal([]byte(commit.Document), &doc)
	if err != nil {
		return err
	}

	switch doc.Type {
	case DocumentTypeCreate, DocumentTypeTimeline, DocumentTypeCollection:
		return handleInsert(ctx, db, commit, doc)
	case DocumentTypeDelete:
		return handleDelete(ctx, db, doc)
	default:
		return fmt.Errorf("unknown document type: %s", doc.Type)
	}
}
