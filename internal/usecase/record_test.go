package usecase

import (
	"context"
	"testing"

	"github.com/totegamma/concrnt-playground"
)

type mockRecordRepo struct {
	created concrnt.SignedDocument
	deleted string
}

func (m *mockRecordRepo) Create(ctx context.Context, sd concrnt.SignedDocument) error {
	m.created = sd
	return nil
}

func (m *mockRecordRepo) GetValue(ctx context.Context, uri string) (any, error) { return nil, nil }
func (m *mockRecordRepo) Delete(ctx context.Context, uri string) error {
	m.deleted = uri
	return nil
}

func TestRecordUsecaseCommitCreate(t *testing.T) {
	repo := &mockRecordRepo{}
	uc := NewRecordUsecase(repo)

	sd := concrnt.SignedDocument{Document: `{"value":{"x":1}}`}
	input := CommitInput{
		Document: concrnt.Document[any]{Value: map[string]any{"x": 1}},
		Raw:      sd,
	}

	if err := uc.Commit(context.Background(), input); err != nil {
		t.Fatalf("commit create failed: %v", err)
	}

	if repo.created.Document == "" {
		t.Fatalf("expected create to be called")
	}
}

func TestRecordUsecaseCommitDelete(t *testing.T) {
	repo := &mockRecordRepo{}
	uc := NewRecordUsecase(repo)

	uri := "cc://example/id"
	input := CommitInput{
		Document: concrnt.Document[any]{},
		Delete:   &uri,
	}

	if err := uc.Commit(context.Background(), input); err != nil {
		t.Fatalf("commit delete failed: %v", err)
	}

	if repo.deleted != uri {
		t.Fatalf("expected delete %s got %s", uri, repo.deleted)
	}
}
