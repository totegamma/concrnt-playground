package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/concrnt/chunkline"
)

type ChunklineUsecase struct {
	repo    ChunklineRepository
	gateway ChunklineGateway
}

func NewChunklineUsecase(repo ChunklineRepository, gateway ChunklineGateway) *ChunklineUsecase {
	return &ChunklineUsecase{
		repo:    repo,
		gateway: gateway,
	}
}

func (uc *ChunklineUsecase) GetChunklineManifest(ctx context.Context, uri string) (*chunkline.Manifest, error) {
	return uc.repo.GetChunklineManifest(ctx, uri)
}

func (uc *ChunklineUsecase) LookupLocalItrs(ctx context.Context, uris []string, chunkID int64) (map[string]int64, error) {
	return uc.repo.LookupLocalItrs(ctx, uris, chunkID)
}

func (uc *ChunklineUsecase) LoadLocalBody(ctx context.Context, uri string, chunkID int64) ([]chunkline.BodyItem, error) {
	return uc.repo.LoadLocalBody(ctx, uri, chunkID)
}

func (uc *ChunklineUsecase) GetRecent(ctx context.Context, uris []string, until time.Time, limit int) ([]chunkline.BodyItem, error) {

	if uc.gateway == nil {
		return nil, fmt.Errorf("chunkline gateway not configured")
	}

	items, err := uc.gateway.QueryDescending(ctx, uris, until, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query descending: %v", err)
	}

	return items, nil
}
