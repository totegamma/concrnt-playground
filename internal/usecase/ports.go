package usecase

import (
	"context"
	"time"

	"github.com/concrnt/chunkline"

	"github.com/totegamma/concrnt-playground"
	"github.com/totegamma/concrnt-playground/internal/domain"
)

// RecordRepository defines storage operations for records/commits.
type RecordRepository interface {
	Create(ctx context.Context, sd concrnt.SignedDocument) error
	GetValue(ctx context.Context, uri string) (any, error)
	Delete(ctx context.Context, uri string) error
}

// ChunklineRepository defines storage operations for chunkline timelines.
type ChunklineRepository interface {
	GetChunklineManifest(ctx context.Context, uri string) (*chunkline.Manifest, error)
	LookupLocalItrs(ctx context.Context, uris []string, chunkID int64) (map[string]int64, error)
	LoadLocalBody(ctx context.Context, uri string, chunkID int64) ([]chunkline.BodyItem, error)
}

// ChunklineGateway encapsulates external timeline resolution.
type ChunklineGateway interface {
	QueryDescending(ctx context.Context, uris []string, until time.Time, limit int) ([]chunkline.BodyItem, error)
}

// EntityRepository defines persistence/lookup for entities.
type EntityRepository interface {
	Register(ctx context.Context, entity domain.Entity, meta domain.EntityMeta) error
	Get(ctx context.Context, ccid string, resolver string) (domain.Entity, error)
}

// ServerRepository defines persistence/lookup for remote servers.
type ServerRepository interface {
	Resolve(ctx context.Context, identifier, hint string) (domain.Server, error)
}
