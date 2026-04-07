package usecase

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/pkg/errors"

	"github.com/totegamma/concrnt-playground"
	"github.com/totegamma/concrnt-playground/cdid"
	"github.com/totegamma/concrnt-playground/client"
	"github.com/totegamma/concrnt-playground/impl/interop"
	"github.com/totegamma/concrnt-playground/internal/domain"
	"github.com/totegamma/concrnt-playground/internal/service"
	"github.com/totegamma/concrnt-playground/internal/utils"
	"github.com/totegamma/concrnt-playground/policy"
	"github.com/totegamma/concrnt-playground/schemas"
)

// RecordRepository defines storage operations for records/commits.
type RecordRepository interface {
	CreateRecord(ctx context.Context, documentID string, sd concrnt.SignedDocument) (string, error)
	CreateAssociation(ctx context.Context, documentID string, parsed concrnt.Document[any], sd concrnt.SignedDocument) error
	Acknowledge(ctx context.Context, documentID string, sd concrnt.SignedDocument) (string, error)
	UnAcknowledge(ctx context.Context, documentID string, sd concrnt.SignedDocument) error
	Delete(ctx context.Context, sd concrnt.SignedDocument) (string, error)

	GetSignedDocument(ctx context.Context, uri string) (*concrnt.SignedDocument, error)
	GetHierarchicalRecordPolicies(ctx context.Context, uri string) ([][]concrnt.Policy, error)

	GetDistributions(ctx context.Context, uri string) ([]string, error)

	GetAcknowledgeRecords(ctx context.Context, from, to, context string) ([]concrnt.Document[schemas.Acknowledge], error)
	GetAcknowledgeRecordCounts(ctx context.Context, from, to, context string) (map[string]int64, error)
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

func (uc *RecordUsecase) Commit(ctx context.Context, sd concrnt.SignedDocument) (*concrnt.SignedDocument, error) {
	ctx, span := tracer.Start(ctx, "Usecase.Record.Commit")
	defer span.End()

	var doc concrnt.Document[any]
	err := json.Unmarshal([]byte(sd.Document), &doc)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	// validate
	switch sd.Proof.Type {
	case concrnt.ProofTypeEcrecover:
		if sd.Proof.Signature == nil {
			err := errors.New("[sub] signature is required for ecrecover proof")
			span.RecordError(err)
			return nil, err
		}
		signatureBytes, err := hex.DecodeString(*sd.Proof.Signature)
		if err != nil {
			span.RecordError(err)
			return nil, err
		}
		err = concrnt.VerifySignature([]byte(sd.Document), signatureBytes, doc.Author)
		if err != nil {
			span.RecordError(err)
			return nil, err
		}
	case concrnt.ProofTypeDocumentReference:
		if sd.Proof.Href == nil {
			err := errors.New("href is required for document-reference proof")
			span.RecordError(err)
			return nil, err
		}
		// TODO: 参照先のドキュメントの検証
	case concrnt.ProofTypeSubkey:
		if sd.Proof.Signature == nil {
			err := errors.New("[sub] signature is required for subkey proof")
			span.RecordError(err)
			return nil, err
		}

		if sd.Proof.Key == nil {
			err := errors.New("[sub] key is required for subkey proof")
			span.RecordError(err)
			return nil, err
		}

		var subKeyDoc concrnt.Document[schemas.Subkey]
		err := uc.client.GetRecord(ctx, *sd.Proof.Key, &subKeyDoc)
		if err != nil {
			span.RecordError(err)
			return nil, err
		}

		signatureBytes, err := hex.DecodeString(*sd.Proof.Signature)
		if err != nil {
			span.RecordError(err)
			return nil, err
		}

		err = concrnt.VerifySignature([]byte(sd.Document), signatureBytes, subKeyDoc.Value.CKID)
		if err != nil {
			span.RecordError(err)
			return nil, err
		}

	default:
		err := errors.New("unsupported proof type: " + sd.Proof.Type)
		span.RecordError(err)
		return nil, err
	}

	var referrer *string
	entityRef, ok := ctx.Value(interop.RequesterCtxKey).(concrnt.Entity)
	if ok {
		referrer = &entityRef.CCID
	}

	requesterID := doc.Author
	requester, err := uc.entity.Get(ctx, requesterID, referrer)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	// accept
	switch doc.Schema {
	// 特殊なスキーマの場合の処理
	case schemas.DeleteURL:
		return uc.deleteRecord(ctx, requester, sd)
	case schemas.AcknowledgeURL:
		return uc.acknowledge(ctx, requester, doc, sd)
	case schemas.UnAcknowledgeURL:
		return uc.unacknowledge(ctx, requester, doc, sd)
	default:
		// Associateフィールドがあれば通常Recordではない
		if doc.Associate != nil {
			return uc.createAssociation(ctx, requester, doc, sd)
		} else { // 通常Record
			return uc.createRecord(ctx, requester, doc, sd)
		}
	}
}

func (uc *RecordUsecase) deleteRecord(ctx context.Context, requester domain.Entity, sd concrnt.SignedDocument) (*concrnt.SignedDocument, error) {
	ctx, span := tracer.Start(ctx, "Usecase.Record.Delete")
	defer span.End()

	var deletedoc concrnt.Document[schemas.Delete]
	err := json.Unmarshal([]byte(sd.Document), &deletedoc)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	target, err := uc.repo.GetSignedDocument(ctx, string(deletedoc.Value))
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	targetDoc := concrnt.Document[any]{}
	err = json.Unmarshal([]byte(target.Document), &targetDoc)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	stack, err := uc.repo.GetHierarchicalRecordPolicies(ctx, string(deletedoc.Value))
	if err != nil {
		span.RecordError(err)
		return nil, err
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
		return nil, err
	}

	resultURI, err := uc.repo.Delete(ctx, sd)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}
	// signal
	err = uc.signal.Publish(ctx, resultURI, concrnt.Event{
		Type: "deleted",
		URI:  resultURI,
	})
	if err != nil {
		fmt.Printf("Error publishing signal for delete: %v\n", err)
		span.RecordError(err)
		return nil, err
	}

	return target, nil
}

func (uc *RecordUsecase) createRecord(ctx context.Context, requester domain.Entity, parsed concrnt.Document[any], sd concrnt.SignedDocument) (*concrnt.SignedDocument, error) {
	ctx, span := tracer.Start(ctx, "Usecase.Record.CreateRecord")
	defer span.End()

	hash := concrnt.GetHash([]byte(sd.Document))
	hash10 := [10]byte{}
	copy(hash10[:], hash[:10])
	documentID := cdid.New(hash10, parsed.CreatedAt).String()

	resultURI, err := uc.repo.CreateRecord(ctx, documentID, sd)
	if err != nil {
		span.RecordError(err)
		return nil, err
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
		return nil, err
	}

	for _, destURI := range *parsed.Distributes {
		dest, err := concrnt.ParseCCURI(destURI)
		if err != nil {
			fmt.Printf("Error parsing memberOf URI: %v\n", err)
			span.RecordError(err)
			continue
		}

		key, err := url.JoinPath(destURI, documentID)
		if err != nil {
			fmt.Printf("Error joining path for distribution: %v\n", err)
			span.RecordError(err)
			continue
		}

		distDoc := concrnt.Document[schemas.Reference]{
			Key: key,
			Value: schemas.Reference{
				Href: resultURI,
			},
			Author:    parsed.Author,
			Schema:    schemas.ReferenceURL,
			CreatedAt: time.Now(),
		}
		docBytes, err := json.Marshal(distDoc)
		if err != nil {
			span.RecordError(err)
			return nil, err
		}
		distSD := concrnt.SignedDocument{
			Document: string(docBytes),
			Proof: concrnt.Proof{
				Type: "document-reference",
				Href: &resultURI,
			},
			References: map[string]any{
				requester.CCKV(): requester.Native(),
			},
		}

		err = uc.client.Commit(ctx, dest.Owner, distSD)
		if err != nil {
			fmt.Printf("Error committing memberOf item: %v\n", err)
			span.RecordError(err)
			continue
		}
	}

	return &sd, nil
}

func (uc *RecordUsecase) createAssociation(ctx context.Context, requester domain.Entity, parsed concrnt.Document[any], sd concrnt.SignedDocument) (*concrnt.SignedDocument, error) {
	ctx, span := tracer.Start(ctx, "Usecase.Record.CreateAssociation")
	defer span.End()

	hash := concrnt.GetHash([]byte(sd.Document))
	hash10 := [10]byte{}
	copy(hash10[:], hash[:10])
	documentID := cdid.New(hash10, parsed.CreatedAt).String()

	target := *parsed.Associate
	targetURI, err := concrnt.ParseCCURI(*parsed.Associate)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	ccfs := concrnt.ComposeCCURI("ccfs", targetURI.Owner, documentID)

	isLocal, err := uc.entity.IsLocalByCCID(ctx, targetURI.Owner)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	created := false

	if isLocal {
		err := uc.repo.CreateAssociation(ctx, documentID, parsed, sd)
		if err != nil {
			span.RecordError(err)
			return nil, err
		}

		created = true

		for _, destURI := range *parsed.Distributes {
			dest, err := concrnt.ParseCCURI(destURI)
			if err != nil {
				fmt.Printf("Error parsing memberOf URI: %v\n", err)
				span.RecordError(err)
				continue
			}

			key, err := url.JoinPath(destURI, documentID)
			if err != nil {
				fmt.Printf("Error joining path for distribution: %v\n", err)
				span.RecordError(err)
				continue
			}

			distDoc := concrnt.Document[schemas.Reference]{
				Key: key,
				Value: schemas.Reference{
					Href: ccfs,
				},
				Author:    parsed.Author,
				Schema:    schemas.ReferenceURL,
				CreatedAt: time.Now(),
			}
			docBytes, err := json.Marshal(distDoc)
			if err != nil {
				span.RecordError(err)
				return nil, err
			}
			distSD := concrnt.SignedDocument{
				Document: string(docBytes),
				Proof: concrnt.Proof{
					Type: "document-reference",
					Href: &ccfs,
				},
				References: map[string]any{
					requester.CCKV(): requester.Native(),
				},
			}

			err = uc.client.Commit(ctx, dest.Owner, distSD)
			if err != nil {
				fmt.Printf("Error committing memberOf item: %v\n", err)
				span.RecordError(err)
				continue
			}
		}
	}

	// signal
	distributions, err := uc.repo.GetDistributions(ctx, target)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	targetSD, ok := sd.References[target].(concrnt.SignedDocument)
	// TODO: add signature validation
	if !ok {
		targetSDr, err := uc.repo.GetSignedDocument(ctx, target)
		if err != nil {
			span.RecordError(err)
			return nil, err
		}
		targetSD = *targetSDr
	}

	notificationChannels := append(distributions, target)
	for _, channel := range notificationChannels {

		host, err := uc.client.ResolveResourceHost(ctx, channel)
		if err != nil {
			fmt.Printf("Error resolving resource host for channel %s: %v\n", channel, err)
			span.RecordError(err)
			continue
		}

		if host == uc.config.FQDN { // local
			err = uc.signal.Publish(ctx, channel, concrnt.Event{
				Type: "associated",
				URI:  target,
				References: map[string]concrnt.SignedDocument{
					ccfs: sd,
				},
			})
			if err != nil {
				fmt.Printf("Error publishing signal for association: %v\n", err)
				span.RecordError(err)
				return nil, err
			}
		} else { // remote
			if !created {
				continue
			}

			sd := concrnt.SignedDocument{
				Document: sd.Document,
				Proof:    sd.Proof,
				References: map[string]any{
					requester.CCKV(): requester.Native(),
					target:           targetSD,
				},
			}

			uc.client.Commit(ctx, host, sd)
		}
	}

	return &sd, nil
}

func (uc *RecordUsecase) acknowledge(ctx context.Context, requester domain.Entity, parsed concrnt.Document[any], sd concrnt.SignedDocument) (*concrnt.SignedDocument, error) {
	ctx, span := tracer.Start(ctx, "Usecase.Record.Acknowledge")
	defer span.End()

	hash := concrnt.GetHash([]byte(sd.Document))
	hash10 := [10]byte{}
	copy(hash10[:], hash[:10])
	documentID := cdid.New(hash10, parsed.CreatedAt).String()

	_, err := uc.repo.Acknowledge(ctx, documentID, sd)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	return &sd, nil
}

func (uc *RecordUsecase) unacknowledge(ctx context.Context, requester domain.Entity, parsed concrnt.Document[any], sd concrnt.SignedDocument) (*concrnt.SignedDocument, error) {
	ctx, span := tracer.Start(ctx, "Usecase.Record.UnAcknowledge")

	hash := concrnt.GetHash([]byte(sd.Document))
	hash10 := [10]byte{}
	copy(hash10[:], hash[:10])
	documentID := cdid.New(hash10, parsed.CreatedAt).String()

	err := uc.repo.UnAcknowledge(ctx, documentID, sd)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	return &sd, nil
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

	requester, ok := ctx.Value(interop.RequesterCtxKey).(concrnt.Entity)
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

func (uc *RecordUsecase) GetAcknowledgeRecords(ctx context.Context, from, to, context string) ([]concrnt.Document[schemas.Acknowledge], error) {
	return uc.repo.GetAcknowledgeRecords(ctx, from, to, context)
}

func (uc *RecordUsecase) GetAcknowledgeRecordCounts(ctx context.Context, from, to, context string) (map[string]int64, error) {
	return uc.repo.GetAcknowledgeRecordCounts(ctx, from, to, context)
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
