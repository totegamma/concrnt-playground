package providers

import (
	"github.com/bradfitz/gomemcache/memcache"
	"gorm.io/gorm"

	"github.com/totegamma/concrnt-playground/client"
	"github.com/totegamma/concrnt-playground/internal/config"
	"github.com/totegamma/concrnt-playground/internal/infrastructure/database"
	"github.com/totegamma/concrnt-playground/internal/infrastructure/gateway"
)

// NewDatabase opens a Postgres connection using the configured DSN.
func NewDatabase(conf config.Server) (*gorm.DB, error) {
	return database.NewPostgres(conf.PostgresDsn)
}

// MigrateDatabase applies migrations for the application models.
func MigrateDatabase(db *gorm.DB) error {
	return database.MigratePostgres(db)
}

// NewMemcache creates a memcache client.
func NewMemcache(addr string) *memcache.Client {
	return memcache.New(addr)
}

// NewClient constructs the HTTP client used to talk to other nodes.
func NewClient(addr string) *client.Client {
	return client.New(addr)
}

// NewChunklineGateway constructs the gateway backed by the HTTP client.
func NewChunklineGateway(cl *client.Client) *gateway.ChunklineGateway {
	return gateway.NewChunklineGateway(cl)
}
