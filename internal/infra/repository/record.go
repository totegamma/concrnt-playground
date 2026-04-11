package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/zeebo/xxh3"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/totegamma/concrnt-playground"
	"github.com/totegamma/concrnt-playground/internal/domain"
	"github.com/totegamma/concrnt-playground/internal/infra/database/models"
	"github.com/totegamma/concrnt-playground/internal/utils"
	"github.com/totegamma/concrnt-playground/schemas"
)

type RecordRepository struct {
	db *gorm.DB
}

func NewRecordRepository(db *gorm.DB) *RecordRepository {
	return &RecordRepository{db: db}
}

func (r *RecordRepository) CreateRecord(ctx context.Context, documentID string, sd concrnt.SignedDocument) (string, error) {
	ctx, span := tracer.Start(ctx, "Repository.Record.CreateRecord")
	defer span.End()

	var doc concrnt.Document[any]
	err := json.Unmarshal([]byte(sd.Document), &doc)
	if err != nil {
		span.RecordError(err)
		return "", err
	}

	parsed, err := concrnt.ParseCCURI(doc.Key)
	if err != nil {
		span.RecordError(err)
		return "", err
	}

	if parsed.Scheme != "cckv" {
		err := fmt.Errorf("invalid key: document key scheme must be cckv")
		span.RecordError(err)
		return "", err
	}

	owner := parsed.Owner

	var policies *string

	if doc.Policies != nil {
		policiesBytes, err := json.Marshal(doc.Policies)
		if err != nil {
			return "", err
		}
		policiesStr := string(policiesBytes)
		policies = &policiesStr
	}

	distributions := []string{}
	if doc.Distributes != nil {
		distributions = *doc.Distributes
	}

	record := models.Record{
		DocumentID:    documentID,
		Owner:         owner,
		Schema:        doc.Schema,
		Policies:      policies,
		Distributions: distributions,
		CDate:         time.Now(),
	}

	proof, err := json.Marshal(sd.Proof)
	if err != nil {
		span.RecordError(err)
		return "", err
	}

	commitLog := models.CommitLog{
		ID:       documentID,
		Document: sd.Document,
		Proof:    string(proof),
	}

	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {

		if err := tx.Clauses(clause.OnConflict{
			DoNothing: true,
		}).Create(&commitLog).Error; err != nil {
			span.RecordError(err)
			return err
		}

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

		if err := tx.Clauses(clause.OnConflict{
			DoNothing: true,
		}).Create(&record).Error; err != nil {
			span.RecordError(err)
			return err
		}

		// update RecordKey
		var oldRecordKey models.RecordKey
		err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("uri = ?", doc.Key).
			Take(&oldRecordKey).Error
		if err != nil && err != gorm.ErrRecordNotFound {
			span.RecordError(err)
			return err
		}

		// ParentのRecordKeyを探す
		parentRK, err := getOrCreateParentRecordKey(ctx, tx, doc.Key)
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
			URI:      doc.Key,
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

		return nil
	})

	if err != nil {
		return "", err
	}

	return doc.Key, nil

}

func (r *RecordRepository) CreateAssociation(ctx context.Context, documentID string, parsed concrnt.Document[any], sd concrnt.SignedDocument) error {
	ctx, span := tracer.Start(ctx, "Repository.Record.CreateAssociation")
	defer span.End()

	targetURI, err := concrnt.ParseCCURI(*parsed.Associate)
	if err != nil {
		span.RecordError(err)
		return err
	}

	if targetURI.Scheme != "cckv" {
		err := fmt.Errorf("invalid associate: document associate scheme must be cckv")
		span.RecordError(err)
		return err
	}

	owner := targetURI.Owner

	targetRK, err := GetRecordKeyByURI(ctx, r.db, *parsed.Associate)
	if err != nil {
		span.RecordError(err)
		return err
	}

	uniqueKey := owner + parsed.Author + *parsed.Associate
	if parsed.AssociationVariant != nil {
		uniqueKey += *parsed.AssociationVariant
	}
	uniqueHash := xxh3.HashString(uniqueKey)

	association := models.Association{
		TargetID:   targetRK.ID,
		DocumentID: documentID,
		Unique:     fmt.Sprintf("%x", uniqueHash),

		Owner:   owner,
		Author:  parsed.Author,
		Variant: parsed.AssociationVariant,
		Schema:  parsed.Schema,
		CDate:   time.Now(),
	}

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

	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {

		if err := tx.Clauses(clause.OnConflict{
			DoNothing: true,
		}).Create(&commitLog).Error; err != nil {
			span.RecordError(err)
			return err
		}

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

		if err := tx.Create(&association).Error; err != nil {
			span.RecordError(err)
			return err
		}

		return nil
	})

	return err
}

func (r *RecordRepository) Acknowledge(ctx context.Context, documentID string, sd concrnt.SignedDocument) (string, error) {
	ctx, span := tracer.Start(ctx, "Repository.Record.Acknowledge")
	defer span.End()

	var doc concrnt.Document[schemas.Acknowledge]
	err := json.Unmarshal([]byte(sd.Document), &doc)
	if err != nil {
		span.RecordError(err)
		return "", err
	}

	parsed, err := concrnt.ParseCCURI(*doc.Associate)
	if err != nil {
		span.RecordError(err)
		return "", err
	}

	if parsed.Scheme != "cckv" {
		err := fmt.Errorf("invalid associate: document associate scheme must be cckv")
		span.RecordError(err)
		return "", err
	}

	to := parsed.Owner
	from := doc.Author

	ack := models.Ack{
		From:       from,
		To:         to,
		Context:    doc.Value.Context,
		DocumentID: documentID,
	}

	proof, err := json.Marshal(sd.Proof)
	if err != nil {
		span.RecordError(err)
		return "", err
	}

	commitLog := models.CommitLog{
		ID:       documentID,
		Document: sd.Document,
		Proof:    string(proof),
	}

	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {

		if err := tx.Clauses(clause.OnConflict{
			DoNothing: true,
		}).Create(&commitLog).Error; err != nil {
			span.RecordError(err)
			return err
		}

		err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "commit_log_id"}, {Name: "owner"}},
			DoNothing: true,
		}).Create(&models.CommitOwner{
			CommitLogID: commitLog.ID,
			Owner:       from,
		}).Error
		if err != nil {
			span.RecordError(err)
			return err
		}
		err = tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "commit_log_id"}, {Name: "owner"}},
			DoNothing: true,
		}).Create(&models.CommitOwner{
			CommitLogID: commitLog.ID,
			Owner:       to,
		}).Error
		if err != nil {
			span.RecordError(err)
			return err
		}

		if err := tx.Create(&ack).Error; err != nil {
			span.RecordError(err)
			return err
		}

		return nil
	})

	ccfs := concrnt.ComposeCCURI("ccfs", to, documentID)

	return ccfs, err
}

func (r *RecordRepository) UnAcknowledge(ctx context.Context, documentID string, sd concrnt.SignedDocument) error {
	ctx, span := tracer.Start(ctx, "Repository.Record.Unacknowledge")
	defer span.End()

	var doc concrnt.Document[schemas.Acknowledge]
	err := json.Unmarshal([]byte(sd.Document), &doc)
	if err != nil {
		span.RecordError(err)
		return err
	}

	parsed, err := concrnt.ParseCCURI(*doc.Associate)
	if err != nil {
		span.RecordError(err)
		return err
	}

	if parsed.Scheme != "cckv" {
		err := fmt.Errorf("invalid associate: document associate scheme must be cckv")
		span.RecordError(err)
		return err
	}

	to := parsed.Owner
	from := doc.Author

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

	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {

		if err := tx.Clauses(clause.OnConflict{
			DoNothing: true,
		}).Create(&commitLog).Error; err != nil {
			span.RecordError(err)
			return err
		}

		err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "commit_log_id"}, {Name: "owner"}},
			DoNothing: true,
		}).Create(&models.CommitOwner{
			CommitLogID: commitLog.ID,
			Owner:       from,
		}).Error
		if err != nil {
			span.RecordError(err)
			return err
		}
		err = tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "commit_log_id"}, {Name: "owner"}},
			DoNothing: true,
		}).Create(&models.CommitOwner{
			CommitLogID: commitLog.ID,
			Owner:       to,
		}).Error
		if err != nil {
			span.RecordError(err)
			return err
		}

		err = tx.Model(&models.Ack{}).
			Where("acks.from = ? AND acks.to = ? AND acks.context = ?", from, to, doc.Value.Context).
			Update("valid", false).
			Update("document_id", documentID).
			Error
		if err != nil {
			span.RecordError(err)
			return err
		}

		return nil
	})

	return err
}

func (r *RecordRepository) GetHierarchicalRecordPolicies(ctx context.Context, uri string) ([][]concrnt.Policy, error) {
	ctx, span := tracer.Start(ctx, "Repository.Record.GetHierarchicalRecordPolicies")
	defer span.End()

	parsed, err := concrnt.ParseCCURI(uri)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	cckv := uri
	if parsed.Scheme == "ccfs" {
		var rk models.RecordKey
		err = r.db.WithContext(ctx).
			Joins("JOIN records r ON r.document_id = record_keys.record_id").
			Where("r.document_id = ?", parsed.CDID).
			Take(&rk).Error
		if err != nil {
			span.RecordError(err)
			return nil, errors.Join(domain.NotFoundError{Resource: uri}, err)
		}
		cckv = rk.URI
	}

	hierarchy := []string{}
	currentURI := cckv
	for {
		hierarchy = append([]string{currentURI}, hierarchy...)
		parentURI, err := url.JoinPath(currentURI, "..")
		if err != nil {
			span.RecordError(err)
			return nil, err
		}
		if parentURI == currentURI {
			break
		}
		currentURI = parentURI
	}

	type tuple struct {
		Uri    string  `gorm:"column:uri"`
		Policy *string `gorm:"column:policy"`
	}

	var entries []tuple
	err = r.db.WithContext(ctx).
		Model(&models.Record{}).
		Select("rk.uri AS uri, records.policies AS policy").
		Joins("JOIN record_keys rk ON rk.record_id = records.document_id").
		Where("rk.uri IN ?", hierarchy).
		Find(&entries).Error
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	policyMap := make(map[string]*string)
	for _, res := range entries {
		policyMap[res.Uri] = res.Policy
	}

	policies := [][]concrnt.Policy{}
	for i := len(hierarchy) - 1; i >= 0; i-- {
		uri := hierarchy[i]
		if policy, ok := policyMap[uri]; ok {
			if policy != nil {
				var policyDocs []concrnt.Policy
				err := json.Unmarshal([]byte(*policy), &policyDocs)
				if err != nil {
					span.RecordError(err)
					return nil, err
				}
				policies = append(policies, policyDocs)
			}
		}
	}

	return policies, nil
}

func (r *RecordRepository) GetSignedDocument(ctx context.Context, uri string) (*concrnt.SignedDocument, error) {
	ctx, span := tracer.Start(ctx, "Repository.Record.GetSignedDocument")
	defer span.End()

	parsed, err := concrnt.ParseCCURI(uri)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	switch parsed.Scheme {
	case "cckv":
		var recordKey models.RecordKey
		err = r.db.WithContext(ctx).Preload("Record").
			Preload("Record.Document").
			Where("uri = ?", uri).
			Take(&recordKey).Error
		if err != nil {
			span.RecordError(err)
			return nil, errors.Join(domain.NotFoundError{Resource: uri}, err)
		}

		var proof concrnt.Proof
		err = json.Unmarshal([]byte(recordKey.Record.Document.Proof), &proof)
		if err != nil {
			span.RecordError(err)
			return nil, err
		}

		ccfs := concrnt.ComposeCCURI("ccfs", parsed.Owner, parsed.CDID)

		return &concrnt.SignedDocument{
			CCKV:     &uri,
			CCFS:     &ccfs,
			Document: recordKey.Record.Document.Document,
			Proof:    proof,
		}, nil

	case "ccfs":
		var commitLog models.CommitLog
		err = r.db.WithContext(ctx).
			Where("id = ?", parsed.CDID).
			Take(&commitLog).Error
		if err != nil {
			span.RecordError(err)
			return nil, errors.Join(domain.NotFoundError{Resource: uri}, err)
		}

		var cckv *string = nil
		var recordKey models.RecordKey
		err = r.db.WithContext(ctx).
			Preload("Record").
			Where("record_id = ?", commitLog.ID).
			Take(&models.RecordKey{}).Error
		if err == nil {
			cckv = &recordKey.URI
		}

		var proof concrnt.Proof
		err = json.Unmarshal([]byte(commitLog.Proof), &proof)
		if err != nil {
			span.RecordError(err)
			return nil, err
		}

		return &concrnt.SignedDocument{
			CCFS:     &uri,
			CCKV:     cckv,
			Document: commitLog.Document,
			Proof:    proof,
		}, nil
	default:
		err := fmt.Errorf("unsupported uri scheme: %s", parsed.Scheme)
		span.RecordError(err)
		return nil, err
	}
}

func (r *RecordRepository) Delete(ctx context.Context, sd concrnt.SignedDocument) (string, error) {
	ctx, span := tracer.Start(ctx, "Repository.Record.Delete")
	defer span.End()

	var doc concrnt.Document[schemas.Delete]
	err := json.Unmarshal([]byte(sd.Document), &doc)
	if err != nil {
		span.RecordError(err)
		return "", err
	}

	target := string(doc.Value)
	parsed, err := concrnt.ParseCCURI(target)
	if err != nil {
		span.RecordError(err)
		return "", err
	}

	switch parsed.Scheme {
	case "cckv":
		var recordKey models.RecordKey
		err = r.db.WithContext(ctx).Preload("Record").
			Preload("Record.Document").
			Where("uri = ?", target).
			Take(&recordKey).Error
		if err != nil {
			span.RecordError(err)
			return "", err
		}

		id := recordKey.Record.DocumentID
		err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := tx.Delete(&models.CommitLog{}, "id = ?", id).Error; err != nil {
				span.RecordError(err)
				return err
			}
			return nil
		})
		if err != nil {
			span.RecordError(err)
			return "", err
		}

		return target, nil

	case "ccfs":
		err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := tx.Delete(&models.CommitLog{}, "id = ?", parsed.CDID).Error; err != nil {
				span.RecordError(err)
				return err
			}
			return nil
		})
		if err != nil {
			span.RecordError(err)
			return "", err
		}
		return target, nil
	default:
		err := fmt.Errorf("unsupported uri scheme: %s", parsed.Scheme)
		span.RecordError(err)
		return "", err
	}
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

	if parsed.Path == "" || parsed.Path == "/" {
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

func (r *RecordRepository) GetDistributions(ctx context.Context, uri string) ([]string, error) {
	ctx, span := tracer.Start(ctx, "Repository.Record.GetDistributions")
	defer span.End()

	parsed, err := concrnt.ParseCCURI(uri)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	switch parsed.Scheme {
	case "cckv":
		var recordKey models.RecordKey
		err = r.db.WithContext(ctx).Preload("Record").
			Preload("Record.Document").
			Where("uri = ?", uri).
			Take(&recordKey).Error
		if err != nil {
			span.RecordError(err)
			return nil, errors.Join(domain.NotFoundError{Resource: uri}, err)
		}
		return recordKey.Record.Distributions, nil
	case "ccfs":
		var record models.Record
		err = r.db.WithContext(ctx).
			Where("document_id = ?", parsed.CDID).
			Take(&record).Error
		if err != nil {
			span.RecordError(err)
			return nil, errors.Join(domain.NotFoundError{Resource: uri}, err)
		}
		return record.Distributions, nil
	default:
		err := fmt.Errorf("unsupported uri scheme: %s", parsed.Scheme)
		span.RecordError(err)
		return nil, err
	}
}

func (r *RecordRepository) GetAssociatedRecords(
	ctx context.Context,
	targetURI, schema, variant, author string,
) ([]concrnt.SignedDocument, error) {
	ctx, span := tracer.Start(ctx, "Repository.Record.GetAssociatedRecords")
	defer span.End()

	var associations []models.Association

	query := r.db.WithContext(ctx).
		Model(&models.Association{}).
		Preload("Document").
		Joins("JOIN record_keys rk ON rk.id = associations.target_id").
		Where("rk.uri = ?", targetURI)

	if schema != "" {
		query = query.Where("associations.schema = ?", schema)
	}
	if variant != "" {
		query = query.Where("associations.variant = ?", variant)
	}
	if author != "" {
		query = query.Where("associations.author = ?", author)
	}

	if err := query.Find(&associations).Error; err != nil {
		return nil, err
	}

	sds := make([]concrnt.SignedDocument, len(associations))
	for i, assoc := range associations {
		var proof concrnt.Proof
		err := json.Unmarshal([]byte(assoc.Document.Proof), &proof)
		if err != nil {
			span.RecordError(err)
			return nil, err
		}

		ccfs := concrnt.ComposeCCURI("ccfs", assoc.Owner, assoc.DocumentID)

		sds[i] = concrnt.SignedDocument{
			CCFS:     &ccfs,
			Document: assoc.Document.Document,
			Proof:    proof,
		}
	}

	return sds, nil
}

func (r *RecordRepository) GetAssociatedRecordCountsBySchema(ctx context.Context, targetURI string) (map[string]int64, error) {
	ctx, span := tracer.Start(ctx, "Repository.Record.GetAssociatedRecordCountsBySchema")
	defer span.End()

	var counts []struct {
		Schema string
		Count  int64
	}

	err := r.db.WithContext(ctx).
		Model(&models.Association{}).
		Select("schema, COUNT(*) AS count").
		Joins("JOIN record_keys rk ON rk.id = associations.target_id").
		Where("rk.uri = ?", targetURI).
		Group("schema").
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

	var counts []struct {
		Variant  string
		Count    int64
		MinCDate time.Time
	}

	err := r.db.WithContext(ctx).
		Model(&models.Association{}).
		Select("variant, COUNT(*) AS count, MIN(c_date) AS min_c_date").
		Joins("JOIN record_keys rk ON rk.id = associations.target_id").
		Where("rk.uri = ?", targetURI).
		Group("variant").
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
) ([]concrnt.SignedDocument, error) {
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

	if limit > 0 {
		query = query.Limit(limit)
	}

	if err := query.Preload("Record.Document").Find(&rks).Error; err != nil {
		span.RecordError(err)
		return nil, err
	}

	sds := make([]concrnt.SignedDocument, 0, len(rks))
	for _, rk := range rks {
		var proof concrnt.Proof
		err := json.Unmarshal([]byte(rk.Record.Document.Proof), &proof)
		if err != nil {
			span.RecordError(err)
			return nil, err
		}

		ccfs := concrnt.ComposeCCURI("ccfs", rk.Record.Owner, rk.Record.DocumentID)

		sds = append(sds, concrnt.SignedDocument{
			CCKV:     &rk.URI,
			CCFS:     &ccfs,
			Document: rk.Record.Document.Document,
			Proof:    proof,
		})
	}

	return sds, nil
}

func (r *RecordRepository) GetAcknowledgeRecords(ctx context.Context, from, to, context string) ([]concrnt.SignedDocument, error) {
	ctx, span := tracer.Start(ctx, "Repository.Record.GetAcknowledgeRecords")
	defer span.End()

	var commits []models.CommitLog
	query := r.db.WithContext(ctx).
		Model(&models.Ack{}).
		Select("commit_logs.*").
		Joins("JOIN commit_logs ON commit_logs.id = acks.document_id")

	if from != "" {
		query = query.Where("acks.from = ?", from)
	}
	if to != "" {
		query = query.Where("acks.to = ?", to)
	}
	if context != "" {
		query = query.Where("acks.context = ?", context)
	}

	err := query.Order("acks.c_date ASC").Find(&commits).Error
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	result := make([]concrnt.SignedDocument, len(commits))
	for i, commit := range commits {
		var proof concrnt.Proof
		err := json.Unmarshal([]byte(commit.Proof), &proof)
		if err != nil {
			span.RecordError(err)
			return nil, err
		}

		ccfs := concrnt.ComposeCCURI("ccfs", to, commit.ID)

		result[i] = concrnt.SignedDocument{
			CCFS:     &ccfs,
			Document: commit.Document,
			Proof:    proof,
		}
	}

	return result, nil
}

func (r *RecordRepository) GetAcknowledgeRecordCounts(ctx context.Context, from, to, context string) (map[string]int64, error) {
	ctx, span := tracer.Start(ctx, "Repository.Record.GetAcknowledgeRecordCounts")
	defer span.End()

	type result struct {
		Context string
		Count   int64
	}

	var results []result

	query := r.db.WithContext(ctx).
		Model(&models.Ack{}).
		Select("context, COUNT(*) AS count")

	if from != "" {
		query = query.Where("from = ?", from)
	}
	if to != "" {
		query = query.Where("to = ?", to)
	}
	if context != "" {
		query = query.Where("context = ?", context)
	}

	err := query.Group("context").Scan(&results).Error
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	counts := make(map[string]int64)
	for _, r := range results {
		counts[r.Context] = r.Count
	}

	return counts, nil
}

func (r *RecordRepository) GetAllCommitLogs(ctx context.Context, owner string) ([]concrnt.SignedDocument, error) {
	ctx, span := tracer.Start(ctx, "Repository.Record.GetAllCommitLogs")
	defer span.End()

	var commitLogs []models.CommitLog
	err := r.db.WithContext(ctx).
		Joins("JOIN commit_owners co ON co.commit_log_id = commit_logs.id").
		Where("co.owner = ?", owner).
		Order("commit_logs.c_date ASC").
		Find(&commitLogs).Error
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	sds := make([]concrnt.SignedDocument, len(commitLogs))
	for i, cl := range commitLogs {
		var proof concrnt.Proof
		err := json.Unmarshal([]byte(cl.Proof), &proof)
		if err != nil {
			span.RecordError(err)
			return nil, err
		}
		sds[i] = concrnt.SignedDocument{
			Document: cl.Document,
			Proof:    proof,
		}
	}

	return sds, nil
}
