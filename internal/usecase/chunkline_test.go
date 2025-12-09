package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/concrnt/chunkline"
)

type mockChunkRepo struct {
	manifestURI string
}

func (m *mockChunkRepo) GetChunklineManifest(ctx context.Context, uri string) (*chunkline.Manifest, error) {
	m.manifestURI = uri
	return &chunkline.Manifest{Version: "1.0"}, nil
}
func (m *mockChunkRepo) LookupLocalItrs(ctx context.Context, uris []string, chunkID int64) (map[string]int64, error) {
	return map[string]int64{uris[0]: chunkID}, nil
}
func (m *mockChunkRepo) LoadLocalBody(ctx context.Context, uri string, chunkID int64) ([]chunkline.BodyItem, error) {
	return []chunkline.BodyItem{{Href: uri}}, nil
}

type mockChunkGateway struct {
	queries [][]string
}

func (m *mockChunkGateway) QueryDescending(ctx context.Context, uris []string, until time.Time, limit int) ([]chunkline.BodyItem, error) {
	m.queries = append(m.queries, uris)
	return []chunkline.BodyItem{{Href: uris[0]}}, nil
}

func TestChunklineUsecaseGetRecent(t *testing.T) {
	repo := &mockChunkRepo{}
	gw := &mockChunkGateway{}
	uc := NewChunklineUsecase(repo, gw)

	uris := []string{"cc://alice/tl"}
	items, err := uc.GetRecent(context.Background(), uris, time.Now(), 5)
	if err != nil {
		t.Fatalf("get recent failed: %v", err)
	}
	if len(items) != 1 || items[0].Href != uris[0] {
		t.Fatalf("unexpected items %+v", items)
	}
	if len(gw.queries) == 0 {
		t.Fatalf("gateway not invoked")
	}
}
