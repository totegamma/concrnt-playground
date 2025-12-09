package usecase

import (
	"context"
	"testing"

	"github.com/totegamma/concrnt-playground"
	"github.com/totegamma/concrnt-playground/internal/domain"
	"github.com/totegamma/concrnt-playground/schemas"
)

type mockEntityRepo struct {
	entity domain.Entity
	meta   domain.EntityMeta
}

func (m *mockEntityRepo) Register(ctx context.Context, e domain.Entity, meta domain.EntityMeta) error {
	m.entity = e
	m.meta = meta
	return nil
}
func (m *mockEntityRepo) Get(ctx context.Context, ccid string, resolver string) (domain.Entity, error) {
	return domain.Entity{}, nil
}

func TestEntityUsecaseRegister(t *testing.T) {
	repo := &mockEntityRepo{}
	uc := NewEntityUsecase(repo)

	doc := concrnt.Document[schemas.Affiliation]{
		Author: "ccid:abc",
		Value:  schemas.Affiliation{Domain: "example.com"},
	}
	input := EntityRegisterInput{
		Document:  doc,
		Raw:       `{"author":"ccid:abc"}`,
		Signature: "sig",
		Meta:      domain.EntityMeta{ID: "ccid:abc"},
	}

	if err := uc.Register(context.Background(), input); err != nil {
		t.Fatalf("register failed: %v", err)
	}

	if repo.entity.ID != doc.Author {
		t.Fatalf("expected entity author %s got %s", doc.Author, repo.entity.ID)
	}
	if repo.entity.AffiliationDocument == "" || repo.entity.AffiliationSignature == "" {
		t.Fatalf("expected raw doc/signature to be stored")
	}
}
