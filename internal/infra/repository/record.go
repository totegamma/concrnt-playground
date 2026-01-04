package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/zeebo/xxh3"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/totegamma/concrnt-playground"
	"github.com/totegamma/concrnt-playground/cdid"
	"github.com/totegamma/concrnt-playground/internal/domain"
	"github.com/totegamma/concrnt-playground/internal/infra/database/models"
	"github.com/totegamma/concrnt-playground/internal/service"
	"github.com/totegamma/concrnt-playground/internal/utils"
	"github.com/totegamma/concrnt-playground/schemas"
)

type RecordRepository struct {
	db     *gorm.DB
	signal *service.SignalService
}

func NewRecordRepository(db *gorm.DB, signal *service.SignalService) *RecordRepository {
	return &RecordRepository{db: db, signal: signal}
}

func (r *RecordRepository) CreateRecord(ctx context.Context, sd concrnt.SignedDocument) error {
	ctx, span := tracer.Start(ctx, "Repository.Record.CreateRecord")
	defer span.End()

	var doc concrnt.Document[any]
	err := json.Unmarshal([]byte(sd.Document), &doc)
	if err != nil {
		span.RecordError(err)
		return err
	}

	hash := concrnt.GetHash([]byte(sd.Document))
	hash10 := [10]byte{}
	copy(hash10[:], hash[:10])
	createdAt := doc.CreatedAt
	documentID := cdid.New(hash10, createdAt).String()

	owner := doc.Author
	if doc.Owner != nil {
		owner = *doc.Owner
	}

	record := models.Record{
		DocumentID: documentID,
		Owner:      owner,
		Schema:     doc.Schema,
		CDate:      time.Now(),
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {

		proof, err := json.Marshal(sd.Proof)
		if err != nil {
			span.RecordError(err)
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
			span.RecordError(err)
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
				span.RecordError(err)
				return err
			}
		}

		if err := tx.Clauses(clause.OnConflict{
			DoNothing: true,
		}).Create(&record).Error; err != nil {
			span.RecordError(err)
			return err
		}

		key := doc.Key
		if strings.Contains(key, "{cdid}") {
			key = strings.ReplaceAll(key, "{cdid}", documentID)
		}
		uri := concrnt.ComposeCCURI(owner, key)

		var oldRecordKey models.RecordKey
		err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("uri = ?", concrnt.ComposeCCURI(owner, key)).
			Take(&oldRecordKey).Error
		if err != nil && err != gorm.ErrRecordNotFound {
			span.RecordError(err)
			return err
		}

		// ParentのRecordKeyを探す
		parentRK, err := getOrCreateParentRecordKey(ctx, tx, concrnt.ComposeCCURI(owner, key))
		if err != nil {
			span.RecordError(err)
			return err
		}

		var pid *int64
		if parentRK != nil {
			pid = &parentRK.ID
		}

		// RecordKeyを作る
		rk := models.RecordKey{
			URI:      uri,
			ParentID: pid,
			RecordID: &documentID,
		}

		err = tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "uri"}},
			DoUpdates: clause.Assignments(map[string]any{"record_id": documentID}),
		}).Create(&rk).Error
		if err != nil {
			span.RecordError(err)
			return err
		}

		// 古いRecordKeyが指していたCommitのGCフラグを立て、Recordは消す
		if oldRecordKey.RecordID != nil && *oldRecordKey.RecordID != documentID {
			if err := tx.Model(&models.CommitLog{}).
				Where("id = ?", oldRecordKey.RecordID).
				Update("gc_candidate", true).Error; err != nil {
				span.RecordError(err)
				return err
			}
			if err := tx.Delete(&models.Record{}, "document_id = ?", oldRecordKey.RecordID).Error; err != nil {
				span.RecordError(err)
				return err
			}
		}

		// Distribute
		if doc.MemberOf != nil {
			for _, memberOfURI := range *doc.MemberOf {
				memberOwner, key, err := concrnt.ParseCCURI(memberOfURI)
				if err != nil {
					fmt.Printf("Error parsing memberOf URI: %v\n", err)
					span.RecordError(err)
					continue
				}
				path := path.Join(key, documentID)

				document := concrnt.Document[schemas.Reference]{
					Key: path,
					Value: schemas.Reference{
						Href: uri,
					},
					Author:    owner,
					Owner:     &memberOwner,
					Schema:    schemas.ReferenceURL,
					CreatedAt: time.Now(),
				}
				docBytes, err := json.Marshal(document)
				if err != nil {
					span.RecordError(err)
					return err
				}
				sd := concrnt.SignedDocument{
					Document: string(docBytes),
					Proof: concrnt.Proof{
						Type: "document-reference",
						Href: &uri,
					},
				}
				err = r.CreateRecord(ctx, sd)
				if err != nil {
					fmt.Printf("Error creating memberOf item: %v\n", err)
					continue
				}
			}
		}

		// signal
		err = r.signal.Publish(ctx, uri, concrnt.Event{
			Type: "created",
			URI:  uri,
			SD:   &sd,
		})
		if err != nil {
			fmt.Printf("Error publishing signal: %v\n", err)
			span.RecordError(err)
			return err
		}

		return nil
	})
}

func (r *RecordRepository) CreateAssociation(ctx context.Context, sd concrnt.SignedDocument) error {
	ctx, span := tracer.Start(ctx, "Repository.Record.CreateAssociation")
	defer span.End()

	var doc concrnt.Document[any]
	err := json.Unmarshal([]byte(sd.Document), &doc)
	if err != nil {
		span.RecordError(err)
		return err
	}

	hash := concrnt.GetHash([]byte(sd.Document))
	hash10 := [10]byte{}
	copy(hash10[:], hash[:10])
	createdAt := doc.CreatedAt
	documentID := cdid.New(hash10, createdAt).String()

	owner := doc.Author
	if doc.Owner != nil {
		owner = *doc.Owner
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {

		proof, err := json.Marshal(sd.Proof)
		if err != nil {
			span.RecordError(err)
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
			span.RecordError(err)
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
				span.RecordError(err)
				return err
			}
		}

		targetRK, err := GetRecordKeyByURI(ctx, tx, *doc.Associate)
		if err != nil {
			span.RecordError(err)
			return err
		}

		uniqueKey := owner + *doc.Associate
		if doc.AssociationVariant != nil {
			uniqueKey += *doc.AssociationVariant
		}
		uniqueHash := xxh3.HashString(uniqueKey)

		association := models.Association{
			TargetID:   targetRK.ID,
			DocumentID: documentID,
			Unique:     fmt.Sprintf("%x", uniqueHash),

			Owner:  owner,
			Schema: doc.Schema,
			Value:  sd.Document,
			CDate:  time.Now(),
		}
		if err := tx.Create(&association).Error; err != nil {
			span.RecordError(err)
			return err
		}

		// signal
		err = r.signal.Publish(ctx, targetRK.URI, concrnt.Event{
			Type: "associated",
			URI:  targetRK.URI,
			SD:   &sd,
		})
		if err != nil {
			fmt.Printf("Error publishing signal: %v\n", err)
			span.RecordError(err)
			return err
		}

		return nil
	})
}

func (r *RecordRepository) CreateAck(ctx context.Context, sd concrnt.SignedDocument) error {
	return fmt.Errorf("not implemented")
}

func (r *RecordRepository) GetDocument(ctx context.Context, uri string) (*concrnt.Document[any], error) {
	ctx, span := tracer.Start(ctx, "Repository.Record.GetDocument")
	defer span.End()

	commit, err := getCommitByURI(ctx, r.db, uri)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	var doc concrnt.Document[any]
	err = json.Unmarshal([]byte(commit.Document), &doc)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	return &doc, nil
}

func (r *RecordRepository) GetSignedDocument(ctx context.Context, uri string) (*concrnt.SignedDocument, error) {
	ctx, span := tracer.Start(ctx, "Repository.Record.GetSignedDocument")
	defer span.End()

	commit, err := getCommitByURI(ctx, r.db, uri)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	var proof concrnt.Proof
	err = json.Unmarshal([]byte(commit.Proof), &proof)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	sd := concrnt.SignedDocument{
		Document: commit.Document,
		Proof:    proof,
	}

	return &sd, nil
}

func (r *RecordRepository) Delete(ctx context.Context, sd concrnt.SignedDocument) error {
	ctx, span := tracer.Start(ctx, "Repository.Record.Delete")
	defer span.End()

	var doc concrnt.Document[schemas.Delete]
	err := json.Unmarshal([]byte(sd.Document), &doc)
	if err != nil {
		span.RecordError(err)
		return err
	}

	record, err := getRecordByURI(ctx, r.db, string(doc.Value))
	if err != nil {
		span.RecordError(err)
		return err
	}

	id := record.DocumentID

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&models.CommitLog{}, "id = ?", id).Error; err != nil {
			span.RecordError(err)
			return err
		}
		return nil
	})
}

func getCommitByURI(ctx context.Context, db *gorm.DB, uri string) (*models.CommitLog, error) {
	ctx, span := tracer.Start(ctx, "Repository.Record.getCommitByURI")
	defer span.End()

	_, key, err := concrnt.ParseCCURI(uri)
	if err != nil {
		span.RecordError(err)
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

func getOrCreateParentRecordKey(ctx context.Context, db *gorm.DB, uri string) (*models.RecordKey, error) {
	ctx, span := tracer.Start(ctx, "Repository.Record.getOrCreateParentRecordKey")
	defer span.End()

	parentURI, err := url.JoinPath(uri, "..")
	if err != nil {
		span.RecordError(err)
		return nil, err
	}
	parsed, err := url.Parse(parentURI)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}
	if parsed.Path == "/" {
		return nil, nil
	}

	parentRK, err := GetRecordKeyByURI(ctx, db, parentURI)
	if err != nil {
		if errors.Is(err, domain.NotFoundError{}) {

			parentID, err := getOrCreateParentRecordKey(ctx, db, parentURI)
			if err != nil {
				span.RecordError(err)
				return nil, err
			}

			var pid *int64
			if parentID != nil {
				pid = &parentID.ID
			}

			newRecordKey := models.RecordKey{
				URI:      parentURI,
				RecordID: nil,
				ParentID: pid,
			}

			err = db.WithContext(ctx).Create(&newRecordKey).Error
			if err != nil {
				span.RecordError(err)
				return nil, err
			}

			return &newRecordKey, nil

		} else {
			span.RecordError(err)
			return nil, err
		}
	}

	return parentRK, nil
}

func getRecordByURI(ctx context.Context, db *gorm.DB, uri string) (*models.Record, error) {
	ctx, span := tracer.Start(ctx, "Repository.Record.getRecordByURI")
	defer span.End()

	_, key, err := concrnt.ParseCCURI(uri)
	if err != nil {
		span.RecordError(err)
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

func GetRecordKeyByURI(ctx context.Context, db *gorm.DB, uri string) (*models.RecordKey, error) {
	ctx, span := tracer.Start(ctx, "Repository.Record.GetRecordKeyByURI")
	defer span.End()

	var recordKey models.RecordKey
	err := db.WithContext(ctx).
		Where("uri = ?", uri).
		Take(&recordKey).Error
	if err != nil {
		span.RecordError(err)
		return nil, domain.NotFoundError{Resource: "record key"}
	}

	return &recordKey, nil
}

func (r *RecordRepository) GetAssociatedRecords(
	ctx context.Context,
	targetURI, schema, variant, author string,
) ([]concrnt.Document[any], error) {
	ctx, span := tracer.Start(ctx, "Repository.Record.GetAssociatedRecords")
	defer span.End()

	targetRK, err := GetRecordKeyByURI(ctx, r.db, targetURI)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	var associations []models.Association

	query := r.db.WithContext(ctx).
		Model(&models.Association{}).
		Preload("Item.Record.Document").
		Joins("JOIN record_keys rk ON rk.id = associations.item_id").
		Joins("JOIN records rec ON rec.document_id = rk.record_id").
		Where("associations.target_id = ?", targetRK.ID)

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
		err := json.Unmarshal([]byte(assoc.Document.Document), &doc)
		if err != nil {
			span.RecordError(err)
			return nil, err
		}
		documents = append(documents, doc)
	}

	return documents, nil
}

func (r *RecordRepository) GetAssociatedRecordCountsBySchema(ctx context.Context, targetURI string) (map[string]int64, error) {
	ctx, span := tracer.Start(ctx, "Repository.Record.GetAssociatedRecordCountsBySchema")
	defer span.End()

	targetRK, err := GetRecordKeyByURI(ctx, r.db, targetURI)
	if err != nil {
		span.RecordError(err)
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
		Where("associations.target_id = ?", targetRK.ID).
		Group("rec.schema").
		Scan(&counts).Error

	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	result := make(map[string]int64)
	for _, c := range counts {
		result[c.Schema] = c.Count
	}

	return result, nil

}

func (r *RecordRepository) GetAssociatedRecordCountsByVariant(ctx context.Context, targetURI, schema string) (*utils.OrderedKVMap[int64], error) {
	ctx, span := tracer.Start(ctx, "Repository.Record.GetAssociatedRecordCountsByVariant")
	defer span.End()

	targetRK, err := GetRecordKeyByURI(ctx, r.db, targetURI)
	if err != nil {
		span.RecordError(err)
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
		Where("associations.target_id = ? AND rec.schema = ?", targetRK.ID, schema).
		Group("rec.variant").
		Order("min_c_date ASC").
		Scan(&counts).Error
	if err != nil {
		span.RecordError(err)
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

func (r *RecordRepository) Query(
	ctx context.Context,
	prefix, schema string,
	since, until *time.Time,
	limit int,
	order string,
) (map[string]concrnt.Document[any], error) {
	ctx, span := tracer.Start(ctx, "Repository.Record.Query")
	defer span.End()

	var rks []models.RecordKey

	query := r.db.WithContext(ctx).
		Model(&models.RecordKey{}).
		Joins("JOIN records r ON r.document_id = record_keys.record_id").
		Where("uri LIKE ?", prefix+"%")

	if schema != "" {
		query = query.Where("r.schema = ?", schema)
	}
	if since != nil {
		query = query.Where("r.c_date >= ?", *since)
	}
	if until != nil {
		query = query.Where("r.c_date <= ?", *until)
	}

	if order == "desc" {
		query = query.Order("r.c_date DESC")
	} else {
		query = query.Order("r.c_date ASC")
	}

	if err := query.Limit(limit).Preload("Record.Document").Find(&rks).Error; err != nil {
		span.RecordError(err)
		return nil, err
	}

	documents := make(map[string]concrnt.Document[any])
	for _, rk := range rks {
		var doc concrnt.Document[any]
		if err := json.Unmarshal([]byte(rk.Record.Document.Document), &doc); err != nil {
			span.RecordError(err)
			return nil, err
		}
		documents[rk.URI] = doc
	}

	return documents, nil
}
