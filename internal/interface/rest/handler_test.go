package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/concrnt/chunkline"
	"github.com/labstack/echo/v4"

	"github.com/totegamma/concrnt-playground"
	"github.com/totegamma/concrnt-playground/internal/config"
	"github.com/totegamma/concrnt-playground/internal/domain"
	"github.com/totegamma/concrnt-playground/internal/usecase"
	"github.com/totegamma/concrnt-playground/schemas"
)

// --- mocks ---

type mockRecordRepo struct {
	deleted string
}

func (m *mockRecordRepo) Create(ctx context.Context, sd concrnt.SignedDocument) error { return nil }
func (m *mockRecordRepo) GetValue(ctx context.Context, uri string) (any, error)       { return "ok", nil }
func (m *mockRecordRepo) Delete(ctx context.Context, uri string) error {
	m.deleted = uri
	return nil
}

type mockChunkRepo struct{}

func (m *mockChunkRepo) GetChunklineManifest(ctx context.Context, uri string) (*chunkline.Manifest, error) {
	return &chunkline.Manifest{Version: "1.0"}, nil
}
func (m *mockChunkRepo) LookupLocalItrs(ctx context.Context, uris []string, chunkID int64) (map[string]int64, error) {
	return map[string]int64{uris[0]: 0}, nil
}
func (m *mockChunkRepo) LoadLocalBody(ctx context.Context, uri string, chunkID int64) ([]chunkline.BodyItem, error) {
	return []chunkline.BodyItem{{Href: uri, Timestamp: time.Now()}}, nil
}

type mockChunkGateway struct{}

func (m *mockChunkGateway) QueryDescending(ctx context.Context, uris []string, until time.Time, limit int) ([]chunkline.BodyItem, error) {
	return []chunkline.BodyItem{{Href: uris[0]}}, nil
}

type mockEntityRepo struct{}

func (m *mockEntityRepo) Register(ctx context.Context, e domain.Entity, meta domain.EntityMeta) error {
	return nil
}
func (m *mockEntityRepo) Get(ctx context.Context, ccid string, resolver string) (domain.Entity, error) {
	return domain.Entity{ID: ccid, Domain: "example.com"}, nil
}

type mockServerRepo struct{}

func (m *mockServerRepo) Resolve(ctx context.Context, identifier, hint string) (domain.Server, error) {
	return domain.Server{WellKnown: concrnt.WellKnownConcrnt{Domain: identifier}}, nil
}

// --- tests ---

func TestHandleCommitDelete(t *testing.T) {
	recRepo := &mockRecordRepo{}
	recordUC := usecase.NewRecordUsecase(recRepo)
	chunkUC := usecase.NewChunklineUsecase(&mockChunkRepo{}, &mockChunkGateway{})
	entityUC := usecase.NewEntityUsecase(&mockEntityRepo{})
	serverUC := usecase.NewServerUsecase(&mockServerRepo{})

	h := NewHandler(config.NodeInfo{}, recordUC, chunkUC, serverUC, entityUC)

	e := echo.New()
	h.RegisterRoutes(e)

	deleteDoc, _ := json.Marshal(concrnt.Document[schemas.Delete]{
		Schema:   ptrStr(schemas.DeleteURL),
		Value:    schemas.Delete("cc://example/id"),
		Author:   "ccid:author",
		CreateAt: time.Now(),
	})

	body, _ := json.Marshal(concrnt.SignedDocument{
		Document: string(deleteDoc),
		Proof:    concrnt.Proof{Type: "sig", Signature: "abc"},
	})

	req := httptest.NewRequest(http.MethodPost, "/commit", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	res := httptest.NewRecorder()

	e.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", res.Code)
	}
	if recRepo.deleted == "" {
		t.Fatalf("expected delete to be invoked")
	}
}

func ptrStr[T ~string](s T) *T { return &s }
