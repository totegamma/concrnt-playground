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
	"github.com/totegamma/concrnt-playground/cdid"
	"github.com/totegamma/concrnt-playground/client"
	"github.com/totegamma/concrnt-playground/internal/domain"
	"github.com/totegamma/concrnt-playground/internal/service"
	"github.com/totegamma/concrnt-playground/internal/utils"
	"github.com/totegamma/concrnt-playground/policy"
	"github.com/totegamma/concrnt-playground/schemas"
)

// RecordRepository defines storage operations for records/commits.
type RecordRepository interface {
	CreateRecord(ctx context.Context, documentID string, sd concrnt.SignedDocument) (string, error)
	CreateAssociation(ctx context.Context, documentID string, sd concrnt.SignedDocument) (string, string, error)
	CreateAck(ctx context.Context, sd concrnt.SignedDocument) error
	Delete(ctx context.Context, sd concrnt.SignedDocument) (string, error)

	GetSignedDocument(ctx context.Context, uri string) (*concrnt.SignedDocument, error)
	GetHierarchicalRecordPolicies(ctx context.Context, uri string) ([][]concrnt.Policy, error)

	GetDistributions(ctx context.Context, uri string) ([]string, error)

	GetAssociatedRecords(ctx context.Context, targetURI, schema, variant, author string) ([]concrnt.Document[any], error)
	GetAssociatedRecordCountsBySchema(ctx context.Context, targetURI string) (map[string]int64, error)
	GetAssociatedRecordCountsByVariant(ctx context.Context, targetURI, schema string) (*utils.OrderedKVMap[int64], error)
	Query(ctx context.Context, prefix, schema string, since, until *time.Time, limit int, order string) ([]concrnt.SignedDocument, error)
}

type RecordUsecase struct {
	repo   RecordRepository
	config *domain.Config
	client *client.Client
	entity *EntityUsecase
	signal *service.SignalService
	policy *service.PolicyService
}

func NewRecordUsecase(
	repo RecordRepository,
	config *domain.Config,
	client *client.Client,
	entity *EntityUsecase,
	signal *service.SignalService,
	policy *service.PolicyService,
) *RecordUsecase {
	return &RecordUsecase{
		repo:   repo,
		config: config,
		client: client,
		entity: entity,
		signal: signal,
		policy: policy,
	}
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
	case concrnt.ProofTypeDocumentReference:
		if sd.Proof.Href == nil {
			err := errors.New("href is required for document-reference proof")
			span.RecordError(err)
			return err
		}
		// TODO: 参照先のドキュメントの検証
	default:
		err := errors.New("unsupported proof type: " + sd.Proof.Type)
		span.RecordError(err)
		return err
	}

	hash := concrnt.GetHash([]byte(sd.Document))
	hash10 := [10]byte{}
	copy(hash10[:], hash[:10])
	documentID := cdid.New(hash10, doc.CreatedAt).String()

	var referrer *string
	if v := ctx.Value(domain.ReferrerCtxKey); v != nil {
		if s, ok := v.(string); ok {
			referrer = &s
		}
	}

	requesterID := doc.Author
	requester, err := uc.entity.Get(ctx, requesterID, referrer)
	if err != nil {
		span.RecordError(err)
		return err
	}

	var resultURI string

	// accept
	switch doc.Schema {
	// 特殊なスキーマの場合の処理
	case schemas.DeleteURL:

		var deletedoc concrnt.Document[schemas.Delete]
		err := json.Unmarshal([]byte(sd.Document), &deletedoc)
		if err != nil {
			span.RecordError(err)
			return err
		}

		target, err := uc.repo.GetSignedDocument(ctx, string(deletedoc.Value))
		if err != nil {
			span.RecordError(err)
			return err
		}

		targetDoc := concrnt.Document[any]{}
		err = json.Unmarshal([]byte(target.Document), &targetDoc)
		if err != nil {
			span.RecordError(err)
			return err
		}

		stack, err := uc.repo.GetHierarchicalRecordPolicies(ctx, string(deletedoc.Value))
		if err != nil {
			span.RecordError(err)
			return err
		}

		err = uc.policy.Eval(
			ctx,
			policy.RequestContext{
				Requester: requester,
				This:      targetDoc,
			},
			stack,
			"net.concrnt.core.commit.delete",
		)
		if err != nil {
			span.RecordError(err)
			return err
		}

		resultURI, err = uc.repo.Delete(ctx, sd)
		if err != nil {
			span.RecordError(err)
			return err
		}
		// signal
		err = uc.signal.Publish(ctx, resultURI, concrnt.Event{
			Type: "deleted",
			URI:  resultURI,
		})
		if err != nil {
			fmt.Printf("Error publishing signal for delete: %v\n", err)
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
			if path.Path == "" { // ack
				err := uc.repo.CreateAck(ctx, sd)
				if err != nil {
					span.RecordError(err)
					return err
				}
			} else { // association
				var targetURI string
				targetURI, resultURI, err = uc.repo.CreateAssociation(ctx, documentID, sd)
				if err != nil {
					span.RecordError(err)
					return err
				}

				// signal
				distributions, err := uc.repo.GetDistributions(ctx, targetURI)
				if err != nil {
					span.RecordError(err)
					return err
				}

				notificationChannels := append(distributions, targetURI)
				for _, channel := range notificationChannels {
					err = uc.signal.Publish(ctx, channel, concrnt.Event{
						Type: "associated",
						URI:  targetURI,
						References: map[string]concrnt.SignedDocument{
							resultURI: sd,
						},
					})
					if err != nil {
						fmt.Printf("Error publishing signal for association: %v\n", err)
						span.RecordError(err)
						return err
					}
				}
			}
		} else { // 通常Record
			resultURI, err = uc.repo.CreateRecord(ctx, documentID, sd)
			if err != nil {
				span.RecordError(err)
				return err
			}
			// signal
			err = uc.signal.Publish(ctx, resultURI, concrnt.Event{
				Type:       "created",
				URI:        resultURI,
				References: map[string]concrnt.SignedDocument{resultURI: sd},
			})
			if err != nil {
				fmt.Printf("Error publishing signal: %v\n", err)
				span.RecordError(err)
				return err
			}

		}
	}

	if resultURI == "" {
		return nil
	}

	// Distribute
	if doc.Distributes != nil {
		for _, memberOfURI := range *doc.Distributes {
			parsed, err := concrnt.ParseCCURI(memberOfURI)
			if err != nil {
				fmt.Printf("Error parsing memberOf URI: %v\n", err)
				span.RecordError(err)
				continue
			}
			path := path.Join(parsed.Key, documentID)

			distDoc := concrnt.Document[schemas.Reference]{
				Key: path,
				Value: schemas.Reference{
					Href: resultURI,
				},
				Author:    doc.Author,
				Owner:     &parsed.Owner,
				Schema:    schemas.ReferenceURL,
				CreatedAt: time.Now(),
			}
			docBytes, err := json.Marshal(distDoc)
			if err != nil {
				span.RecordError(err)
				return err
			}
			distSD := concrnt.SignedDocument{
				Document: string(docBytes),
				Proof: concrnt.Proof{
					Type: "document-reference",
					Href: &resultURI,
				},
			}

			err = uc.client.Commit(ctx, parsed.Owner, distSD, uc.config.FQDN)
			if err != nil {
				fmt.Printf("Error committing memberOf item: %v\n", err)
				span.RecordError(err)
				continue
			}
		}
	}

	return nil
}

func (uc *RecordUsecase) GetSigned(ctx context.Context, uri string) (*concrnt.SignedDocument, error) {
	ctx, span := tracer.Start(ctx, "Usecase.Record.GetSigned")
	defer span.End()

	sd, err := uc.repo.GetSignedDocument(ctx, uri)
	if err != nil {
		return nil, err
	}

	var doc concrnt.Document[any]
	err = json.Unmarshal([]byte(sd.Document), &doc)
	if err != nil {
		return nil, err
	}

	stack, err := uc.repo.GetHierarchicalRecordPolicies(ctx, uri)
	if err != nil {
		span.RecordError(err)
		stack = [][]concrnt.Policy{}
	}

	requester, ok := ctx.Value(domain.RequesterCtxKey).(concrnt.Entity)
	if !ok {
		requester = concrnt.Entity{}
	}

	err = uc.policy.Eval(
		ctx,
		policy.RequestContext{
			Requester: requester,
			This:      doc,
		},
		stack,
		"net.concrnt.core.resolve",
	)

	if err != nil {
		return nil, err
	}

	return sd, nil
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
) ([]concrnt.SignedDocument, error) {
	return uc.repo.Query(ctx, prefix, schema, since, until, limit, order)
}
