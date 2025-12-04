package application

import (
	"context"

	"github.com/concrnt/chunkline"
	"github.com/totegamma/concrnt-playground/internal/infrastructure/repository"
)

type ChunklineApplication struct {
	repo *repository.ChunklineRepository
}

func NewChunklineApplication(repo *repository.ChunklineRepository) *ChunklineApplication {
	return &ChunklineApplication{repo: repo}
}

func (app *ChunklineApplication) GetChunklineManifest(ctx context.Context, uri string) (*chunkline.Manifest, error) {
	return app.repo.GetChunklineManifest(ctx, uri)
}

func (app *ChunklineApplication) LookupLocalItrs(ctx context.Context, uris []string, chunkID int64) (map[string]int64, error) {
	return app.repo.LookupLocalItrs(ctx, uris, chunkID)
}

func (app *ChunklineApplication) LoadLocalBody(ctx context.Context, uri string, chunkID int64) ([]chunkline.BodyItem, error) {
	return app.repo.LoadLocalBody(ctx, uri, chunkID)
}
