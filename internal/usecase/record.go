package usecase

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"time"

	"github.com/pkg/errors"

	"github.com/totegamma/concrnt-playground"
	"github.com/totegamma/concrnt-playground/internal/domain"
	"github.com/totegamma/concrnt-playground/internal/service"
	"github.com/totegamma/concrnt-playground/internal/utils"
	"github.com/totegamma/concrnt-playground/schemas"
)

// RecordRepository defines storage operations for records/commits.
type RecordRepository interface {
	CreateRecord(ctx context.Context, sd concrnt.SignedDocument) (*domain.RecordCreationResult, error)
	CreateAssociation(ctx context.Context, sd concrnt.SignedDocument) error
	CreateAck(ctx context.Context, sd concrnt.SignedDocument) error
	Delete(ctx context.Context, sd concrnt.SignedDocument) error

	GetDocument(ctx context.Context, uri string) (*concrnt.Document[any], error)
	GetSignedDocument(ctx context.Context, uri string) (*concrnt.SignedDocument, error)

	GetAssociatedRecords(ctx context.Context, targetURI, schema, variant, author string) ([]concrnt.Document[any], error)
	GetAssociatedRecordCountsBySchema(ctx context.Context, targetURI string) (map[string]int64, error)
	GetAssociatedRecordCountsByVariant(ctx context.Context, targetURI, schema string) (*utils.OrderedKVMap[int64], error)
	Query(ctx context.Context, prefix, schema string, since, until *time.Time, limit int, order string) (map[string]concrnt.Document[any], error)
}

type RecordUsecase struct {
	repo   RecordRepository
	signal *service.SignalService
}

func NewRecordUsecase(repo RecordRepository, signal *service.SignalService) *RecordUsecase {
	return &RecordUsecase{repo: repo, signal: signal}
}

func (uc *RecordUsecase) Commit(ctx context.Context, sd concrnt.SignedDocument) error {
	ctx, span := tracer.Start(ctx, "Usecase.Record.Commit")
	defer span.End()

	var doc concrnt.Document[any]
	err := json.Unmarshal([]byte(sd.Document), &doc)
	if err != nil {
		span.RecordError(err)
		return err
	}

	// validate
	switch sd.Proof.Type {
	case concrnt.ProofTypeEcrecover:
		if sd.Proof.Signature == nil {
			err := errors.New("[sub] signature is required for ecrecover proof")
			span.RecordError(err)
			return err
		}
		signatureBytes, err := hex.DecodeString(*sd.Proof.Signature)
		if err != nil {
			span.RecordError(err)
			return err
		}
		err = concrnt.VerifySignature([]byte(sd.Document), signatureBytes, doc.Author)
		if err != nil {
			span.RecordError(err)
			return err
		}
	default:
		err := errors.New("unsupported proof type: " + sd.Proof.Type)
		span.RecordError(err)
		return err
	}

	var result *domain.RecordCreationResult

	// accept
	switch doc.Schema {
	// 特殊なスキーマの場合の処理
	case schemas.DeleteURL:
		err := uc.repo.Delete(ctx, sd)
		if err != nil {
			span.RecordError(err)
			return err
		}
	default:
		// Associateフィールドがあれば通常Recordではない
		if doc.Associate != nil {
			path, err := url.Parse(*doc.Associate)
			if err != nil {
				span.RecordError(err)
				return err
			}
			// uriがentityであればAck、そうでなければAssociation
			if path.Path == "" {
				err := uc.repo.CreateAck(ctx, sd)
				if err != nil {
					span.RecordError(err)
					return err
				}
			} else {
				err := uc.repo.CreateAssociation(ctx, sd)
				if err != nil {
					span.RecordError(err)
					return err
				}
			}
		} else {
			result, err = uc.repo.CreateRecord(ctx, sd)
			if err != nil {
				span.RecordError(err)
				return err
			}
			// signal
			err = uc.signal.Publish(ctx, result.URI, concrnt.Event{
				Type: "created",
				URI:  result.URI,
				SD:   &sd,
			})
			if err != nil {
				fmt.Printf("Error publishing signal: %v\n", err)
				span.RecordError(err)
				return err
			}

		}
	}

	if result == nil {
		return nil
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
			path := path.Join(key, result.CDID)

			document := concrnt.Document[schemas.Reference]{
				Key: path,
				Value: schemas.Reference{
					Href: result.URI,
				},
				Author:    result.Owner,
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
					Href: &result.URI,
				},
			}
			distResult, err := uc.repo.CreateRecord(ctx, sd)
			if err != nil {
				fmt.Printf("Error creating memberOf item: %v\n", err)
				continue
			}
			err = uc.signal.Publish(ctx, distResult.URI, concrnt.Event{
				Type: "created",
				URI:  distResult.URI,
				SD:   &sd,
			})
			if err != nil {
				fmt.Printf("Error publishing signal for memberOf item: %v\n", err)
				span.RecordError(err)
				continue
			}
		}
	}

	return nil
}

func (uc *RecordUsecase) Get(ctx context.Context, uri string) (*concrnt.Document[any], error) {
	return uc.repo.GetDocument(ctx, uri)
}

func (uc *RecordUsecase) GetSigned(ctx context.Context, uri string) (*concrnt.SignedDocument, error) {
	return uc.repo.GetSignedDocument(ctx, uri)
}

func (uc *RecordUsecase) GetAssociatedRecords(ctx context.Context, targetURI, schema, variant, author string) ([]concrnt.Document[any], error) {
	return uc.repo.GetAssociatedRecords(ctx, targetURI, schema, variant, author)
}

func (uc *RecordUsecase) GetAssociatedRecordCountsBySchema(ctx context.Context, targetURI string) (map[string]int64, error) {
	return uc.repo.GetAssociatedRecordCountsBySchema(ctx, targetURI)
}

func (uc *RecordUsecase) GetAssociatedRecordCountsByVariant(ctx context.Context, targetURI, schema string) (*utils.OrderedKVMap[int64], error) {
	return uc.repo.GetAssociatedRecordCountsByVariant(ctx, targetURI, schema)
}

func (uc *RecordUsecase) Query(
	ctx context.Context,
	prefix, schema string,
	since, until *time.Time,
	limit int,
	order string,
) (map[string]concrnt.Document[any], error) {
	return uc.repo.Query(ctx, prefix, schema, since, until, limit, order)
}
